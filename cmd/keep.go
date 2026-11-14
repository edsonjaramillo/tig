package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/config"
	"github.com/edsonjaramillo/tig/internal/keep"
)

func newKeepCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "keep",
		Short: "Keep a declared set of repos cloned and up to date",
		Long: `keep reconciles this machine with the keep list: the keep: section of the
tig config, declaring the repos you always want checked out.

It clones missing checkouts, fast-forwards existing ones, and runs hooks.
It is additive only: it never deletes or overwrites anything.`,
	}
	c.AddCommand(
		newKeepCloneCmd(a),
		newKeepEditCmd(a),
		newKeepHooksCmd(a),
		newKeepInitCmd(a),
		newKeepListCmd(a),
		newKeepPullCmd(a),
		newKeepRunCmd(a),
		newKeepStatusCmd(a),
	)
	return c
}

// actionFlags are the common flags of commands that act on kept repos.
type actionFlags struct {
	dryRun  bool
	jobs    int
	noHooks bool
}

func addJobsFlag(c *cobra.Command, f *actionFlags) {
	c.Flags().IntVar(&f.jobs, "jobs", 4, "number of kept repos to process in parallel")
}

func addDryRunFlag(c *cobra.Command, f *actionFlags) {
	c.Flags().BoolVarP(&f.dryRun, "dry-run", "n", false, "print the plan, including hook commands, without executing anything")
}

func addNoHooksFlag(c *cobra.Command, f *actionFlags) {
	c.Flags().BoolVar(&f.noHooks, "no-hooks", false, "skip post-clone and post-pull hooks")
}

// configPath resolves the tig config path from --config and the environment.
func (a *app) configPath() (string, error) {
	return config.Path(a.configFlag)
}

// loadConfig loads the tig config and warns about unsafe permissions.
func (a *app) loadConfig(cmd *cobra.Command) (*config.Config, error) {
	path, err := a.configPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if w := config.PermissionWarning(path); w != "" {
		warn(cmd, "%s", w)
	}
	return cfg, nil
}

func warn(cmd *cobra.Command, format string, args ...any) {
	outf(cmd.ErrOrStderr(), "warning: "+format+"\n", args...)
}

// selectRepos loads the config and selects kept repos by name. Unknown or
// ambiguous names are an error. It returns no repos when the keep list is
// empty.
func (a *app) selectRepos(cmd *cobra.Command, names []string) ([]keep.KeptRepo, error) {
	cfg, err := a.loadConfig(cmd)
	if err != nil {
		return nil, err
	}
	if len(cfg.Keep.Repos) == 0 {
		return nil, nil
	}
	return keep.Select(cfg.Keep.Repos, names)
}

// runOp runs op over the selected kept repos, streams per-repo results, and
// ends with a summary line. It returns nil results when the keep list is
// empty.
func (a *app) runOp(cmd *cobra.Command, names []string, jobs int, op keep.Op) ([]keep.Result, error) {
	repos, err := a.selectRepos(cmd, names)
	if err != nil {
		return nil, err
	}
	out := cmd.OutOrStdout()
	if len(repos) == 0 {
		outln(cmd.ErrOrStderr(), "the keep list is empty; add repos with `tig keep edit`")
		return nil, nil
	}
	emit := func(r keep.Result) { printResult(out, r) }
	results := keep.Run(cmd.Context(), repos, jobs, op, emit)
	outln(out, keep.Summary(results))
	return results, nil
}

// runAction is runOp for commands whose exit code only reflects whether any
// kept repo failed.
func (a *app) runAction(cmd *cobra.Command, names []string, jobs int, op keep.Op) error {
	results, err := a.runOp(cmd, names, jobs, op)
	if err != nil {
		return err
	}
	return failures(results)
}

// completeRepoNames completes repo names from the keep list, leaving out
// names already given. After `--` (as in `keep run`) it falls back to the
// shell's default completion.
func (a *app) completeRepoNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if a.completingAfterDash() {
		return nil, cobra.ShellCompDirectiveDefault
	}
	path, err := a.configPath()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	repos := cfg.Keep.Repos
	given := map[string]bool{}
	for _, arg := range args {
		if sel, err := keep.Select(repos, []string{arg}); err == nil {
			given[sel[0].Name] = true
		}
	}
	shortCount := map[string]int{}
	for _, r := range repos {
		shortCount[strings.ToLower(r.ShortName())]++
	}
	var out []string
	for _, r := range repos {
		if given[r.Name] {
			continue
		}
		desc := "\t" + tildePath(r.Dir)
		if strings.HasPrefix(r.Name, toComplete) {
			out = append(out, r.Name+desc)
		} else if short := r.ShortName(); shortCount[strings.ToLower(short)] == 1 && strings.HasPrefix(short, toComplete) {
			out = append(out, short+desc)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

// completingAfterDash reports whether the word being completed comes after
// a `--`. cmd.ArgsLenAtDash cannot tell during completion, because cobra
// parses the arguments with an extra trailing `--`, so the raw command line is
// checked instead.
func (a *app) completingAfterDash() bool {
	i := slices.IndexFunc(a.argv, func(s string) bool {
		return s == cobra.ShellCompRequestCmd || s == cobra.ShellCompNoDescRequestCmd
	})
	if i < 0 || len(a.argv) < i+2 {
		return false
	}
	before := a.argv[i+1 : len(a.argv)-1] // the last word is being completed
	return slices.Contains(before, "--")
}

func failures(results []keep.Result) error {
	for _, r := range results {
		if r.Failed() {
			return errFailures
		}
	}
	return nil
}

// printResult prints one result: a header line, then buffered output and
// hints, each prefixed so parallel output stays readable.
func printResult(w io.Writer, r keep.Result) {
	header := r.Repo.Name + ": " + r.Label()
	var errLines []string
	if r.Err != nil {
		errLines = strings.Split(strings.TrimRight(r.Err.Error(), "\n"), "\n")
		header += ": " + errLines[0]
		errLines = errLines[1:]
	}
	outln(w, header)
	for _, l := range errLines {
		outln(w, "  "+l)
	}
	if out := strings.TrimRight(r.Output, "\n"); out != "" {
		for _, l := range strings.Split(out, "\n") {
			outln(w, "  │ "+l)
		}
	}
	if r.Hint != "" {
		outln(w, "  "+r.Hint)
	}
}

// tildePath abbreviates the home directory as ~ for display.
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(p, home+string(os.PathSeparator)); ok {
		return "~/" + rest
	}
	return p
}

// outf and outln write user-facing output; write errors (e.g. a closed
// pipe) are deliberately ignored.
func outf(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }

func outln(w io.Writer, args ...any) { _, _ = fmt.Fprintln(w, args...) }
