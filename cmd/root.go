package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// exitError makes tig exit with code after the command has already reported
// what went wrong; Execute prints nothing more for it.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// errFailures signals that a command already reported its failures and only
// needs a non-zero exit code.
var errFailures = exitError{code: 1}

// app holds state shared by all tig commands.
type app struct {
	// configFlag is the persistent --config flag shared by all tig features.
	configFlag string
	// argv is the command line without the program name.
	argv []string
}

// newRootCmd builds the tig command tree for the command line argv (without
// the program name).
func newRootCmd(version string, argv []string) *cobra.Command {
	a := &app{argv: argv}
	root := &cobra.Command{
		Use:   "tig",
		Short: "A CLI of helpers for git and gh workflows",
		Long: `tig is a command-line companion for git and the GitHub CLI (gh).

It wraps common git and gh workflows into simple helper commands,
so everyday tasks like branching, committing, syncing, and working
with pull requests take fewer keystrokes.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.configFlag, "config", "",
		"path to the tig config (default $TIG_CONFIG, $XDG_CONFIG_HOME/tig.yaml, or ~/.config/tig.yaml)")
	root.AddCommand(newConfigCmd(a), newKeepCmd(a))
	root.SetArgs(argv)
	return root
}

// Execute runs the root command and exits non-zero on failure.
func Execute(version string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := newRootCmd(version, os.Args[1:]).ExecuteContext(ctx)
	stop()
	if err != nil {
		var ee exitError
		if errors.As(err, &ee) {
			os.Exit(ee.code)
		}
		outln(os.Stderr, "tig:", err)
		os.Exit(1)
	}
}
