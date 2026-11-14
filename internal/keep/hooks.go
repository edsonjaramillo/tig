package keep

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// HookKind selects which of a kept repo's hooks to run.
type HookKind int

// Hook kinds.
const (
	PostClone HookKind = iota
	PostPull
)

// String returns the keep list key of the hook kind.
func (k HookKind) String() string {
	if k == PostPull {
		return "post_pull"
	}
	return "post_clone"
}

// Hooks returns the kept repo's hooks of the given kind.
func (r KeptRepo) Hooks(k HookKind) []string {
	if k == PostPull {
		return r.PostPull
	}
	return r.PostClone
}

// hookEnv returns the inherited environment plus the TIG_* variables
// describing the kept repo.
func hookEnv(r KeptRepo, defaultBranch string) []string {
	return append(os.Environ(),
		"TIG_REPO_NAME="+r.Name,
		"TIG_DESTINATION="+r.Dir,
		"TIG_DEFAULT_BRANCH="+defaultBranch,
	)
}

// runHooks runs each command with `sh -c` in the checkout, one at a time,
// stopping at the first failure. Hooks get no stdin. Combined output of all
// commands run so far is returned.
func runHooks(ctx context.Context, r KeptRepo, defaultBranch string, cmds []string) (string, error) {
	var out bytes.Buffer
	for _, c := range cmds {
		cmd := exec.CommandContext(ctx, "sh", "-c", c)
		cmd.Dir = r.Dir
		cmd.Env = hookEnv(r, defaultBranch)
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return out.String(), fmt.Errorf("hook %q failed: %w", c, err)
		}
	}
	return out.String(), nil
}

// runArgv runs argv in the checkout with no stdin: directly, or when shell is
// set, as a single `sh -c` script made of argv joined by spaces.
func runArgv(ctx context.Context, r KeptRepo, defaultBranch string, argv []string, shell bool) (string, error) {
	if shell {
		argv = []string{"sh", "-c", strings.Join(argv, " ")}
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = r.Dir
	cmd.Env = hookEnv(r, defaultBranch)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
