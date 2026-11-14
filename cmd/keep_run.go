package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/git"
	"github.com/edsonjaramillo/tig/internal/keep"
)

func newKeepRunCmd(a *app) *cobra.Command {
	var (
		f     actionFlags
		shell bool
	)
	c := &cobra.Command{
		Use:   "run [names...] -- <cmd> [args...]",
		Short: "Run a command in each existing checkout",
		Long: `run executes the command after -- in each existing checkout. Missing
checkouts are skipped with a note.

By default the command runs directly, not through a shell. With --shell the
words after -- are joined with spaces and run as one sh -c script, so pipes,
globs, and $TIG_* variables work; quote the script to keep your own shell
from expanding it first.

When exactly one checkout runs, tig exits with the command's exit code.
Otherwise tig exits 1 if any command failed.`,
		Example: `  tig keep run -- git log -1 --oneline
  tig keep run tig dotfiles -- git status --short
  tig keep run --shell -- 'git log --oneline | head -3'`,
		ValidArgsFunction: a.completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			dash := cmd.ArgsLenAtDash()
			if dash < 0 || dash == len(args) {
				return errors.New("missing command: use tig keep run [names...] -- <cmd> [args...]")
			}
			op := keep.RunOp(keep.Options{DryRun: f.dryRun}, args[dash:], shell)
			results, err := a.runOp(cmd, args[:dash], f.jobs, op)
			if err != nil {
				return err
			}
			if len(results) == 1 && results[0].Failed() {
				if code := git.ExitCode(results[0].Err); code > 0 {
					return exitError{code: code}
				}
			}
			return failures(results)
		},
	}
	addDryRunFlag(c, &f)
	addJobsFlag(c, &f)
	c.Flags().BoolVar(&shell, "shell", false, "run the command as one sh -c script")
	return c
}
