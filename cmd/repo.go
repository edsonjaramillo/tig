package cmd

import "github.com/spf13/cobra"

func newRepoCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "repo",
		Short: "Manage the GitHub repository of the current checkout",
		Long: `repo connects the git repository in the current directory with GitHub,
using the GitHub CLI (gh) and its active account.`,
	}
	c.AddCommand(newRepoCreateCmd(a))
	return c
}
