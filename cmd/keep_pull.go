package cmd

import (
	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/keep"
)

func newKeepPullCmd(a *app) *cobra.Command {
	var f actionFlags
	c := &cobra.Command{
		Use:   "pull [names...]",
		Short: "Fast-forward checkouts on their default branch",
		Long: `pull fetches the default branch from origin and fast-forwards each checkout to
origin/<default branch>. Checkouts that are missing, dirty, or not on their
default branch are skipped with the reason. Post-pull hooks run only when HEAD
moved.`,
		ValidArgsFunction: a.completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAction(cmd, args, f.jobs, keep.PullOp(keep.Options{DryRun: f.dryRun, NoHooks: f.noHooks}))
		},
	}
	addDryRunFlag(c, &f)
	addJobsFlag(c, &f)
	addNoHooksFlag(c, &f)
	return c
}
