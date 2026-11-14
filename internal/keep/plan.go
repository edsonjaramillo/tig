package keep

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/edsonjaramillo/tig/internal/git"
)

// Options control how operations execute.
type Options struct {
	// DryRun inspects checkouts (read-only, no network) and reports the
	// planned steps, including hook commands, without executing them.
	DryRun bool
	// NoHooks skips post-clone and post-pull hooks.
	NoHooks bool
}

func fail(res Result, err error) Result {
	res.Outcome = Failed
	res.Err = err
	return res
}

func skip(res Result, why string) Result {
	res.Outcome = Skipped
	res.Detail = "skipped: " + why
	return res
}

func planned(res Result, outcome Outcome, steps []string) Result {
	res.Outcome = outcome
	res.Output = strings.Join(steps, "\n")
	return res
}

func hookSteps(prefix string, hooks []string) []string {
	steps := make([]string, len(hooks))
	for i, h := range hooks {
		steps[i] = prefix + h
	}
	return steps
}

// onCheckout returns an Op that runs fn only on an existing checkout of the
// kept repo. A destination that is not the kept repo's checkout fails, and a
// missing checkout is skipped.
func onCheckout(fn func(ctx context.Context, r KeptRepo) Result) Op {
	return func(ctx context.Context, r KeptRepo) Result {
		co, err := Inspect(ctx, r)
		if err != nil {
			return fail(Result{}, err)
		}
		if co.Missing {
			return skip(Result{}, "missing")
		}
		return fn(ctx, r)
	}
}

// CloneArgs returns the git arguments used to clone a kept repo.
func CloneArgs(r KeptRepo) []string {
	args := []string{"clone"}
	if r.Branch != "" {
		args = append(args, "-b", r.Branch)
	}
	return append(args, "--", r.CloneURL, r.Dir)
}

// CloneOp clones missing checkouts and runs post-clone hooks on fresh clones.
// A destination holding the right repo is reported as present; anything else
// at the destination is a failure and is never overwritten.
func CloneOp(opts Options) Op {
	return func(ctx context.Context, r KeptRepo) Result {
		var res Result
		co, err := Inspect(ctx, r)
		if err != nil {
			return fail(res, err)
		}
		if !co.Missing {
			res.Outcome = Present
			return res
		}
		hooks := r.PostClone
		if opts.NoHooks {
			hooks = nil
		}
		args := CloneArgs(r)
		if opts.DryRun {
			steps := append([]string{"git " + strings.Join(args, " ")}, hookSteps("hook: ", hooks)...)
			return planned(res, WouldClone, steps)
		}
		if _, err := git.Run(ctx, "", args...); err != nil {
			return fail(res, err)
		}
		res.Outcome = Cloned
		if len(hooks) == 0 {
			return res
		}
		def, _ := DefaultBranch(ctx, r, false)
		if out, err := runHooks(ctx, r, def, hooks); err != nil {
			res = fail(res, err)
			res.Detail = "cloned, hook failed"
			res.Output = out
			res.Hint = "run: tig keep hooks " + r.Name
		}
		return res
	}
}

// HooksOp re-runs the given kind of hooks in existing checkouts.
func HooksOp(opts Options, kind HookKind) Op {
	return onCheckout(func(ctx context.Context, r KeptRepo) Result {
		var res Result
		hooks := r.Hooks(kind)
		if len(hooks) == 0 {
			res.Outcome = NoHooks
			res.Detail = "no " + kind.String() + " hooks"
			return res
		}
		if opts.DryRun {
			return planned(res, WouldRunHooks, hookSteps("hook: ", hooks))
		}
		def, _ := DefaultBranch(ctx, r, false)
		out, err := runHooks(ctx, r, def, hooks)
		if err != nil {
			res = fail(res, err)
			res.Output = out
			return res
		}
		res.Outcome = HooksRan
		return res
	})
}

// PullOp fast-forwards checkouts that are clean and on their default branch,
// running post-pull hooks when HEAD moved.
func PullOp(opts Options) Op {
	return onCheckout(func(ctx context.Context, r KeptRepo) Result {
		var res Result
		dirty, err := isDirty(ctx, r.Dir)
		if err != nil {
			return fail(res, err)
		}
		if dirty {
			return skip(res, "dirty")
		}
		hooks := r.PostPull
		if opts.NoHooks {
			hooks = nil
		}

		def, err := DefaultBranch(ctx, r, !opts.DryRun)
		if err != nil {
			return fail(res, fmt.Errorf("determining default branch: %w", err))
		}
		if def == "" {
			if opts.DryRun {
				steps := []string{
					"git remote set-head origin --auto",
					"git fetch origin <default branch>",
					"git merge --ff-only origin/<default branch> (if on the default branch)",
				}
				return planned(res, WouldPull, append(steps, hookSteps("hook (if HEAD moves): ", hooks)...))
			}
			return fail(res, errors.New("cannot determine default branch; set `branch` in the keep list"))
		}

		cur := currentBranch(ctx, r.Dir)
		if cur != def {
			if cur == "" {
				return skip(res, "detached HEAD, not "+def)
			}
			return skip(res, fmt.Sprintf("on %s, not %s", cur, def))
		}

		if opts.DryRun {
			steps := []string{"git fetch origin " + def, "git merge --ff-only origin/" + def}
			return planned(res, WouldPull, append(steps, hookSteps("hook (if HEAD moves): ", hooks)...))
		}

		// Fetching just the default branch still updates
		// refs/remotes/origin/<def> through the configured refspec.
		if _, err := git.Run(ctx, r.Dir, "fetch", "origin", def); err != nil {
			return fail(res, err)
		}
		ahead, behind, ok, err := divergence(ctx, r.Dir, def)
		if err != nil {
			return fail(res, err)
		}
		if !ok {
			return fail(res, fmt.Errorf("origin/%s does not exist", def))
		}
		if behind == 0 {
			res.Outcome = UpToDate
			return res
		}
		if ahead > 0 {
			return fail(res, fmt.Errorf("diverged from origin/%s (ahead %d, behind %d); not fast-forwarding", def, ahead, behind))
		}

		// Behind and not ahead, so a successful fast-forward moves HEAD.
		if _, err := git.Run(ctx, r.Dir, "merge", "--ff-only", "refs/remotes/origin/"+def); err != nil {
			return fail(res, err)
		}
		res.Outcome = Pulled
		res.Detail = fmt.Sprintf("pulled %d commit%s", behind, plural(behind))
		if len(hooks) == 0 {
			return res
		}
		if out, err := runHooks(ctx, r, def, hooks); err != nil {
			res = fail(res, err)
			res.Detail = "pulled, hook failed"
			res.Output = out
		}
		return res
	})
}

// RunOp runs argv in each existing checkout: directly, or when shell is set,
// as one `sh -c` script made of argv joined by spaces.
func RunOp(opts Options, argv []string, shell bool) Op {
	return onCheckout(func(ctx context.Context, r KeptRepo) Result {
		var res Result
		if opts.DryRun {
			step := strings.Join(argv, " ")
			if shell {
				step = "sh -c " + strconv.Quote(step)
			}
			return planned(res, WouldRun, []string{step})
		}
		def, _ := DefaultBranch(ctx, r, false)
		out, err := runArgv(ctx, r, def, argv, shell)
		res.Output = out
		if err != nil {
			return fail(res, err)
		}
		res.Outcome = Ran
		return res
	})
}

// StatusOp reports worktree and sync state. Without fetch it makes no
// network access.
func StatusOp(fetch bool) Op {
	return func(ctx context.Context, r KeptRepo) Result {
		st := &Status{}
		res := Result{Status: st}
		co, err := Inspect(ctx, r)
		if err != nil {
			return fail(res, err)
		}
		if co.Missing {
			st.Worktree = WorktreeMissing
			res.Outcome = Missing
			return res
		}
		st.Warning = co.Warning

		if fetch {
			if _, err := git.Run(ctx, r.Dir, "fetch", "origin"); err != nil {
				return fail(res, err)
			}
		}
		def, err := DefaultBranch(ctx, r, fetch)
		if err != nil {
			return fail(res, fmt.Errorf("determining default branch: %w", err))
		}

		cur := currentBranch(ctx, r.Dir)
		st.Branch = cur
		if cur == "" {
			st.Branch = "(detached)"
		}

		dirty, err := isDirty(ctx, r.Dir)
		if err != nil {
			return fail(res, err)
		}
		st.Worktree, res.Outcome = WorktreeClean, Clean
		if dirty {
			st.Worktree, res.Outcome = WorktreeDirty, Dirty
		}

		switch {
		case def == "":
			st.Sync = Sync{State: SyncNoUpstream}
		case cur != def:
			st.Sync = Sync{State: SyncOffBranch, Branch: st.Branch}
		default:
			ahead, behind, ok, err := divergence(ctx, r.Dir, def)
			switch {
			case err != nil:
				return fail(res, err)
			case !ok:
				st.Sync = Sync{State: SyncNoUpstream}
			default:
				st.Sync = newSync(ahead, behind)
			}
		}
		return res
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
