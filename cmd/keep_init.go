package cmd

import (
	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/config"
)

func newKeepInitCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Add a commented starter keep section to the tig config",
		Long: `init appends a commented starter keep: block to the tig config, creating the
file (mode 0600) and its directory if needed. It refuses if a keep section
already exists. The existing file content is left untouched.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := a.configPath()
			if err != nil {
				return err
			}
			if err := config.AppendStarterKeep(path); err != nil {
				return err
			}
			outf(cmd.OutOrStdout(), "added a keep section to %s\nedit it with `tig keep edit`\n", path)
			return nil
		},
	}
}
