package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/config"
)

func newKeepEditCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the tig config in $EDITOR and validate it afterwards",
		Long: `edit opens the tig config in $EDITOR (falling back to $VISUAL, then vi).
After the editor exits, the file is validated and any errors are printed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := a.configPath()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				editor = "vi"
			}
			// Run through sh so EDITOR may carry arguments, e.g. "code --wait".
			ed := exec.CommandContext(cmd.Context(), "sh", "-c", editor+` "$1"`, "sh", path)
			ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := ed.Run(); err != nil {
				return fmt.Errorf("editor %q: %w", editor, err)
			}
			if _, err := os.Stat(path); err != nil {
				outf(cmd.ErrOrStderr(), "%s was not saved\n", path)
				return nil
			}
			if _, err := config.Load(path); err != nil {
				outln(cmd.ErrOrStderr(), err)
				return errFailures
			}
			if w := config.PermissionWarning(path); w != "" {
				warn(cmd, "%s", w)
			}
			outf(cmd.OutOrStdout(), "%s is valid\n", path)
			return nil
		},
	}
}
