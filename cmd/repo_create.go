package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/edsonjaramillo/tig/internal/gh"
	"github.com/edsonjaramillo/tig/internal/git"
)

var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func newRepoCreateCmd(_ *app) *cobra.Command {
	var private bool
	c := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a GitHub repo for the current checkout and set it as origin",
		Long: `create makes a GitHub repository under the active gh account and adds it as
the origin remote (HTTPS) of the git repository in the current directory.

It must run at the root of a git repository (the directory containing .git).
The repo is named after the current directory unless a name is given, and is
public unless --private is set. It refuses if origin is already set or if the
repo already exists on the account. Nothing is pushed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(dir, ".git")); errors.Is(err, fs.ErrNotExist) {
				return errors.New("no .git here: run this at the root of a git repository")
			} else if err != nil {
				return err
			}

			name := filepath.Base(dir)
			if len(args) == 1 {
				name = args[0]
			}
			if !repoNamePattern.MatchString(name) || name == "." || name == ".." {
				return fmt.Errorf("invalid repo name %q: use letters, digits, '.', '-' and '_' (no owner)", name)
			}

			if url, err := git.Run(ctx, dir, "remote", "get-url", "origin"); err == nil {
				return fmt.Errorf("origin is already set to %s", url)
			} else if git.ExitCode(err) != 2 {
				return err
			}

			acct, err := gh.ActiveAccount(ctx)
			if err != nil {
				return err
			}
			fullName := acct.Login + "/" + name
			exists, err := gh.RepoExists(ctx, fullName)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("%s already exists on %s", fullName, acct.Host)
			}

			if err := gh.CreateRepo(ctx, name, private); err != nil {
				return err
			}
			visibility := "public"
			if private {
				visibility = "private"
			}
			out := cmd.OutOrStdout()
			outf(out, "created %s repo %s\n", visibility, fullName)

			remote := git.Remote{Host: acct.Host, Path: fullName}.HTTPSURL()
			if _, err := git.Run(ctx, dir, "remote", "add", "origin", remote); err != nil {
				return fmt.Errorf("created %s but could not set origin: %w", fullName, err)
			}
			outf(out, "set origin to %s\n", remote)
			return nil
		},
	}
	c.Flags().BoolVar(&private, "private", false, "create the repo as private")
	return c
}
