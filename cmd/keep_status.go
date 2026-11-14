package cmd

import (
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/keep"
)

func newKeepStatusCmd(a *app) *cobra.Command {
	var (
		f                       actionFlags
		fetch, dirty, outOfSync bool
	)
	c := &cobra.Command{
		Use:   "status [names...]",
		Short: "Show worktree and sync state of each checkout",
		Long: `status shows, for each kept repo, the checked-out branch, the worktree state
(missing, clean, dirty), and sync against origin/<default branch> (up-to-date,
ahead N, behind N, diverged, no-upstream, off-branch). It makes no network
access unless --fetch is passed.

--dirty shows only dirty checkouts; --out-of-sync shows only checkouts that are
ahead, behind, or diverged. Together they show checkouts matching either.
Failed repos are always shown.`,
		ValidArgsFunction: a.completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			repos, err := a.selectRepos(cmd, args)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			results := keep.Run(cmd.Context(), repos, f.jobs, keep.StatusOp(fetch), nil)
			results, hidden := filterStatus(results, dirty, outOfSync)
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			outln(tw, "REPO\tDESTINATION\tBRANCH\tWORKTREE\tSYNC\tNOTE")
			for _, r := range results {
				st := r.Status
				note := st.Warning
				if r.Err != nil {
					note = "error: " + r.Err.Error()
				}
				outf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					r.Repo.Name, tildePath(r.Repo.Dir), dash(st.Branch), dash(string(st.Worktree)), dash(st.Sync.String()), note)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			outln(out, keep.Summary(results, keep.Count{Label: "hidden", N: hidden}))
			return failures(results)
		},
	}
	addJobsFlag(c, &f)
	c.Flags().BoolVar(&fetch, "fetch", false, "fetch origin before comparing")
	c.Flags().BoolVar(&dirty, "dirty", false, "show only dirty checkouts")
	c.Flags().BoolVar(&outOfSync, "out-of-sync", false, "show only checkouts ahead of, behind, or diverged from origin")
	return c
}

// filterStatus keeps results that are dirty (when dirty is set) or out of
// sync (when outOfSync is set). With neither set, all results are kept.
// Failed results are always kept. It returns the number of results dropped.
func filterStatus(results []keep.Result, dirty, outOfSync bool) ([]keep.Result, int) {
	if !dirty && !outOfSync {
		return results, 0
	}
	kept := results[:0:0]
	for _, r := range results {
		if r.Failed() ||
			(dirty && r.Status.IsDirty()) ||
			(outOfSync && r.Status.IsOutOfSync()) {
			kept = append(kept, r)
		}
	}
	return kept, len(results) - len(kept)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
