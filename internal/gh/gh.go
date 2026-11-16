// Package gh runs the GitHub CLI non-interactively for GitHub-side
// operations such as creating repositories (ADR-0005). Local repository work
// stays with the git CLI (ADR-0001).
package gh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// Error is returned when a gh command exits unsuccessfully.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	sub := ""
	if len(e.Args) > 0 {
		sub = " " + e.Args[0]
	}
	return fmt.Sprintf("gh%s: %s", sub, msg)
}

func (e *Error) Unwrap() error { return e.Err }

// Run executes gh with prompts disabled and returns its trimmed stdout.
func Run(ctx context.Context, args ...string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errors.New("the GitHub CLI (gh) is required: https://cli.github.com")
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "NO_COLOR=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), &Error{Args: args, Stderr: stderr.String(), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Account is the active gh account.
type Account struct {
	// Login is the account's user name.
	Login string
	// Host is the GitHub host the account belongs to, e.g. "github.com".
	Host string
}

// ActiveAccount returns the account gh is currently authenticated as.
func ActiveAccount(ctx context.Context) (Account, error) {
	out, err := Run(ctx, "api", "user", "--jq", ".login + \" \" + .html_url")
	if err != nil {
		return Account{}, fmt.Errorf("finding the active gh account (is `gh auth login` done?): %w", err)
	}
	login, profile, ok := strings.Cut(out, " ")
	if !ok || login == "" {
		return Account{}, fmt.Errorf("unexpected gh api user output %q", out)
	}
	u, err := url.Parse(profile)
	if err != nil || u.Hostname() == "" {
		return Account{}, fmt.Errorf("unexpected gh profile URL %q", profile)
	}
	return Account{Login: login, Host: strings.ToLower(u.Hostname())}, nil
}

// RepoExists reports whether the repository owner/name exists and is visible
// to the active account.
func RepoExists(ctx context.Context, fullName string) (bool, error) {
	_, err := Run(ctx, "api", "repos/"+fullName, "--silent")
	if err == nil {
		return true, nil
	}
	var ge *Error
	if errors.As(err, &ge) && strings.Contains(ge.Stderr, "HTTP 404") {
		return false, nil
	}
	return false, err
}

// CreateRepo creates name under the active account, public unless private.
func CreateRepo(ctx context.Context, name string, private bool) error {
	visibility := "--public"
	if private {
		visibility = "--private"
	}
	_, err := Run(ctx, "repo", "create", name, visibility)
	return err
}
