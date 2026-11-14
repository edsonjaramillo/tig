package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newKeepListCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "list [names...]",
		Short: "Show the keep list as configured (no git calls)",
		Long: `list prints each kept repo's repo name, resolved clone URL, destination,
declared default branch, and hook counts. It reads only the tig config.`,
		ValidArgsFunction: a.completeRepoNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			repos, err := a.selectRepos(cmd, args)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			outln(tw, "REPO\tURL\tDESTINATION\tBRANCH\tHOOKS")
			for _, r := range repos {
				branch := r.Branch
				if branch == "" {
					branch = "(remote HEAD)"
				}
				var hooks []string
				if n := len(r.PostClone); n > 0 {
					hooks = append(hooks, fmt.Sprintf("post_clone %d", n))
				}
				if n := len(r.PostPull); n > 0 {
					hooks = append(hooks, fmt.Sprintf("post_pull %d", n))
				}
				if len(hooks) == 0 {
					hooks = []string{"-"}
				}
				outf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.CloneURL, tildePath(r.Dir), branch, strings.Join(hooks, ", "))
			}
			return tw.Flush()
		},
	}
}
