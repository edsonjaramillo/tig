package cmd

import (
	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/keep"
)

func newKeepCloneCmd(a *app) *cobra.Command {
	var f actionFlags
	c := &cobra.Command{
		Use:   "clone [names...]",
		Short: "Clone missing checkouts and run their post-clone hooks",
		Long: `clone runs git clone for every kept repo whose checkout is missing, then runs
its post-clone hooks. A destination already holding the right repo is reported
as present. Anything else at a destination is an error and is never overwritten.`,
		ValidArgsFunction: a.completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAction(cmd, args, f.jobs, keep.CloneOp(keep.Options{DryRun: f.dryRun, NoHooks: f.noHooks}))
		},
	}
	addDryRunFlag(c, &f)
	addJobsFlag(c, &f)
	addNoHooksFlag(c, &f)
	return c
}
