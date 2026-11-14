package cmd

import (
	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/keep"
)

func newKeepHooksCmd(a *app) *cobra.Command {
	var (
		f        actionFlags
		postPull bool
	)
	c := &cobra.Command{
		Use:   "hooks [names...]",
		Short: "Re-run post-clone (or, with --post-pull, post-pull) hooks in existing checkouts",
		Long: `hooks re-runs the post_clone hooks of kept repos whose checkouts exist, for
example to recover after a hook failed during clone. With --post-pull it runs
the post_pull hooks instead, whether or not HEAD moved.

Hooks run one at a time per checkout, stopping at the first failure.`,
		Example: `  tig keep hooks owner/repo
  tig keep hooks --post-pull`,
		ValidArgsFunction: a.completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := keep.PostClone
			if postPull {
				kind = keep.PostPull
			}
			return a.runAction(cmd, args, f.jobs, keep.HooksOp(keep.Options{DryRun: f.dryRun}, kind))
		},
	}
	addDryRunFlag(c, &f)
	addJobsFlag(c, &f)
	c.Flags().BoolVar(&postPull, "post-pull", false, "run post_pull hooks instead of post_clone hooks")
	return c
}
