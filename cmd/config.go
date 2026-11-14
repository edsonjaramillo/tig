package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/config"
)

func newConfigCmd(a *app) *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Manage the tig config file",
		Long: `config manages the tig config: the single YAML file holding the settings of
every tig feature, each under its own top-level key.`,
	}
	c.AddCommand(
		newConfigCheckCmd(a),
		newConfigEditCmd(a),
		newConfigPathCmd(a),
	)
	return c
}

func newConfigPathCmd(a *app) *cobra.Command {
	var verbose bool
	c := &cobra.Command{
		Use:   "path",
		Short: "Print the path of the tig config",
		Long: `path prints the resolved path of the tig config, whether or not the file
exists. The path comes from --config, then $TIG_CONFIG, then
$XDG_CONFIG_HOME/tig.yaml, then ~/.config/tig.yaml.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, source, err := config.Locate(a.configFlag)
			if err != nil {
				return err
			}
			outln(cmd.OutOrStdout(), path)
			if verbose {
				state := "exists"
				if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
					state = "does not exist"
				}
				outf(cmd.ErrOrStderr(), "from %s; %s\n", source, state)
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&verbose, "verbose", "v", false, "also print (to stderr) which rule chose the path and whether the file exists")
	return c
}

func newConfigCheckCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "check [files...]",
		Short: "Validate the tig config without opening an editor",
		Long: `check validates the tig config, or the given files, the same way every tig
command does when it loads the config. It exits non-zero if any file is
missing or invalid, so it works in CI or as a pre-commit hook.

Group- or world-writable files get a warning but still pass.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := args
			if len(paths) == 0 {
				path, err := a.configPath()
				if err != nil {
					return err
				}
				paths = []string{path}
			}
			failed := false
			for _, p := range paths {
				if _, err := config.Load(p); err != nil {
					outln(cmd.ErrOrStderr(), err)
					failed = true
					continue
				}
				if w := config.PermissionWarning(p); w != "" {
					warn(cmd, "%s", w)
				}
				outf(cmd.OutOrStdout(), "%s is valid\n", p)
			}
			if failed {
				return errFailures
			}
			return nil
		},
	}
}

func newConfigEditCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit the tig config in $VISUAL or $EDITOR, saving it only if valid",
		Long: `edit opens a copy of the tig config in $VISUAL (falling back to $EDITOR, then
vi). When the editor exits, the copy is validated:

  - if it is valid, it atomically replaces the tig config;
  - if it is invalid, the errors are printed and you are asked to edit it
    again; declining discards your changes and leaves the config untouched.

The config's permissions are kept (a new config gets 0600), and if it is a
symlink, the file it points to is replaced, not the link. Without a
terminal, an invalid config is discarded and edit exits non-zero.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := a.configPath()
			if err != nil {
				return err
			}
			return editConfig(cmd, path)
		},
	}
}

// isInteractive reports whether r is a terminal the user can answer
// prompts on. It is a variable so tests can fake a terminal.
var isInteractive = func(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func editConfig(cmd *cobra.Command, path string) error {
	d, err := config.NewDraft(path)
	if err != nil {
		return err
	}
	defer d.Discard()

	errOut := cmd.ErrOrStderr()
	in := bufio.NewReader(cmd.InOrStdin())
	for {
		if err := runEditor(cmd, d.Path); err != nil {
			outln(errOut, "changes discarded")
			return err
		}
		changed, err := d.Changed()
		if err != nil {
			return err
		}
		if !changed {
			outln(cmd.OutOrStdout(), "no changes")
			if editorReturnsEarly() {
				outln(errOut, `hint: if the editor returned before you saved, make it wait (e.g. EDITOR="code --wait")`)
			}
			return nil
		}
		data, err := d.Read()
		if err != nil {
			return err
		}
		_, perr := config.Parse(data, path)
		if perr == nil {
			break
		}
		outln(errOut, perr)
		if !isInteractive(cmd.InOrStdin()) || !askYes(cmd, in, "re-edit? [Y/n] ") {
			outf(errOut, "changes discarded; %s is untouched\n", path)
			return errFailures
		}
	}

	if err := d.Commit(); err != nil {
		return err
	}
	if w := config.PermissionWarning(path); w != "" {
		warn(cmd, "%s", w)
	}
	outf(cmd.OutOrStdout(), "saved %s\n", path)
	return nil
}

// askYes prompts on stderr and reports whether the answer is yes. An empty
// answer is yes; end of input is no.
func askYes(cmd *cobra.Command, in *bufio.Reader, prompt string) bool {
	outf(cmd.ErrOrStderr(), "%s", prompt)
	line, err := in.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true
	}
	return false
}

// editor returns the user's editor command: $VISUAL, then $EDITOR, then vi.
func editor() string {
	for _, k := range []string{"VISUAL", "EDITOR"} {
		if e := strings.TrimSpace(os.Getenv(k)); e != "" {
			return e
		}
	}
	return "vi"
}

// editorReturnsEarly reports whether the editor is a GUI editor that is known
// to return immediately unless told to wait.
func editorReturnsEarly() bool {
	fields := strings.Fields(editor())
	if len(fields) == 0 || slices.Contains(fields, "-w") || slices.Contains(fields, "--wait") {
		return false
	}
	switch filepath.Base(fields[0]) {
	case "code", "code-insiders", "subl", "zed", "cursor", "mate", "atom":
		return true
	}
	return false
}

// runEditor opens file in the user's editor and waits for it to exit.
//
// The editor is deliberately not tied to the command's context: tig's context
// is cancelled on SIGINT, and Ctrl-C inside the editor must not kill it.
func runEditor(cmd *cobra.Command, file string) error {
	e := editor()
	// Run through sh so the editor may carry arguments, e.g. "code --wait".
	ed := exec.Command("sh", "-c", e+` "$1"`, "sh", file)
	ed.Stdout, ed.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
	// Hand the editor the terminal itself; a non-file stdin would be copied
	// into the editor and consumed before the re-edit prompt could read it.
	if f, ok := cmd.InOrStdin().(*os.File); ok {
		ed.Stdin = f
	}
	if err := ed.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", e, err)
	}
	return nil
}
