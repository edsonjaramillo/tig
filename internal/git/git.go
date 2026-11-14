// Package git runs the git CLI non-interactively and parses remote URLs.
//
// It is shared by all tig features and contains only generic git plumbing;
// feature-specific operations live in their own packages (ADR-0001).
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Env returns the environment for git invocations: the inherited environment
// plus settings that make auth problems fail fast instead of prompting.
func Env() []string {
	ssh := "ssh"
	if v := os.Getenv("GIT_SSH_COMMAND"); v != "" {
		ssh = v
	}
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND="+ssh+" -o BatchMode=yes",
	)
}

// Error is returned when a git command exits unsuccessfully.
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
	return fmt.Sprintf("git%s: %s", sub, msg)
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCode returns the exit code of a failed git command, or -1 if err is not
// a git exit error.
func ExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// Command builds a non-interactive git command running in dir (if non-empty).
// Stdin is left unset, so git reads from the null device.
func Command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = Env()
	return cmd
}

// Run executes git in dir and returns its trimmed stdout.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := Command(ctx, dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), &Error{Args: args, Stderr: stderr.String(), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}
