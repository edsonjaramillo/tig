package keep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edsonjaramillo/tig/internal/git"
)

// Checkout describes what is on disk at a kept repo's destination.
type Checkout struct {
	// Missing is true when nothing exists at the destination.
	Missing bool
	// OriginURL is origin's configured URL (empty when Missing).
	OriginURL string
	// Warning notes a protocol mismatch between origin and the keep list.
	Warning string
}

// Inspect examines a kept repo's destination without network access. It
// returns an error when the destination exists but is not a checkout of the
// kept repo (not a directory, not a git checkout, or a different origin).
func Inspect(ctx context.Context, r KeptRepo) (Checkout, error) {
	fi, err := os.Stat(r.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return Checkout{Missing: true}, nil
	}
	if err != nil {
		return Checkout{}, err
	}
	if !fi.IsDir() {
		return Checkout{}, fmt.Errorf("destination %s exists and is not a directory", r.Dir)
	}
	if _, err := os.Lstat(filepath.Join(r.Dir, ".git")); err != nil {
		return Checkout{}, fmt.Errorf("destination %s exists and is not a git checkout", r.Dir)
	}
	top, err := git.Run(ctx, r.Dir, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(top, r.Dir) {
		return Checkout{}, fmt.Errorf("destination %s exists and is not a git checkout", r.Dir)
	}
	origin, err := git.Run(ctx, r.Dir, "config", "--get", "remote.origin.url")
	if err != nil || origin == "" {
		return Checkout{}, fmt.Errorf("checkout at %s has no origin remote", r.Dir)
	}
	remote, err := git.ParseURL(origin)
	if err != nil || remote.Key() != r.Remote.Key() {
		return Checkout{}, fmt.Errorf("checkout at %s has origin %s, expected %s", r.Dir, origin, r.CloneURL)
	}
	c := Checkout{OriginURL: origin}
	if remote.Transport() != r.Remote.Transport() {
		c.Warning = fmt.Sprintf("origin uses %s, keep list says %s", remote.Transport(), r.Remote.Transport())
	}
	return c, nil
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// DefaultBranch returns the kept repo's declared branch, or else the local
// refs/remotes/origin/HEAD. When that is missing and allowNetwork is true it
// runs `git remote set-head origin --auto` once. It returns "" when the
// default branch is unknown.
func DefaultBranch(ctx context.Context, r KeptRepo, allowNetwork bool) (string, error) {
	if r.Branch != "" {
		return r.Branch, nil
	}
	if b := originHead(ctx, r.Dir); b != "" {
		return b, nil
	}
	if !allowNetwork {
		return "", nil
	}
	if _, err := git.Run(ctx, r.Dir, "remote", "set-head", "origin", "--auto"); err != nil {
		return "", err
	}
	return originHead(ctx, r.Dir), nil
}

func originHead(ctx context.Context, dir string) string {
	ref, err := git.Run(ctx, dir, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(ref, "origin/")
}

// currentBranch returns the checked-out branch, or "" when HEAD is detached.
func currentBranch(ctx context.Context, dir string) string {
	b, err := git.Run(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return b
}

// isDirty reports staged or unstaged changes to tracked files.
func isDirty(ctx context.Context, dir string) (bool, error) {
	out, err := git.Run(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// divergence counts commits ahead of and behind origin/<branch>.
// ok is false when the remote-tracking ref does not exist.
func divergence(ctx context.Context, dir, branch string) (ahead, behind int, ok bool, err error) {
	ref := "refs/remotes/origin/" + branch
	if _, err := git.Run(ctx, dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return 0, 0, false, nil
	}
	out, err := git.Run(ctx, dir, "rev-list", "--left-right", "--count", "HEAD..."+ref)
	if err != nil {
		return 0, 0, false, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, false, fmt.Errorf("unexpected rev-list output %q", out)
	}
	ahead, _ = strconv.Atoi(fields[0])
	behind, _ = strconv.Atoi(fields[1])
	return ahead, behind, true, nil
}
