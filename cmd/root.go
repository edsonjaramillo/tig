package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tig",
	Short: "A CLI of helpers for git and gh workflows",
	Long: `tig is a command-line companion for git and the GitHub CLI (gh).

It wraps common git and gh workflows into simple helper commands,
so everyday tasks like branching, committing, syncing, and working
with pull requests take fewer keystrokes.`,
}

func Execute(version string) {
	rootCmd.Version = version
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}
