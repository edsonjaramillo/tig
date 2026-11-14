// Package keep implements the keep list: the declaration of repositories that
// should always exist on this machine, and the operations that reconcile the
// machine with it (clone, hooks, status, pull, run).
//
// keep is additive only: it never deletes or overwrites anything (ADR-0004),
// and it keeps no state of its own (ADR-0002).
package keep

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/edsonjaramillo/tig/internal/git"
)

// Protocols accepted by KeepList.Protocol.
const (
	ProtocolHTTPS = "https"
	ProtocolSSH   = "ssh"
)

// KeepList is the `keep:` section of the tig config.
type KeepList struct {
	// Protocol applies to shorthand repos: "https" (default) or "ssh".
	Protocol string     `yaml:"protocol"`
	Repos    []KeptRepo `yaml:"repos"`
}

// KeptRepo is one entry in the keep list.
type KeptRepo struct {
	Repo        string   `yaml:"repo"`
	Destination string   `yaml:"destination"`
	Branch      string   `yaml:"branch"`
	PostClone   []string `yaml:"post_clone"`
	PostPull    []string `yaml:"post_pull"`

	// Resolved by KeepList.Validate.
	Name     string     `yaml:"-"` // repo name: owner/repo
	CloneURL string     `yaml:"-"`
	Dir      string     `yaml:"-"` // expanded, absolute destination
	Remote   git.Remote `yaml:"-"` // parsed clone URL
}

// Validate checks the keep list and resolves each kept repo's Name, CloneURL,
// and Dir. All problems are reported together.
func (k *KeepList) Validate() error {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	switch k.Protocol {
	case "", ProtocolHTTPS, ProtocolSSH:
	default:
		fail("keep.protocol: %q is not https or ssh", k.Protocol)
	}

	names := map[string]int{}
	dirs := map[string]int{}
	for i := range k.Repos {
		r := &k.Repos[i]
		where := fmt.Sprintf("keep.repos[%d]", i)
		if r.Repo != "" {
			where += " (" + r.Repo + ")"
		}

		if strings.TrimSpace(r.Repo) == "" {
			fail("%s: repo is required", where)
		} else if err := r.resolveRepo(k.Protocol); err != nil {
			fail("%s: %v", where, err)
		} else {
			key := strings.ToLower(r.Name)
			if j, dup := names[key]; dup {
				fail("%s: repo name %s duplicates keep.repos[%d]", where, r.Name, j)
			} else {
				names[key] = i
			}
		}

		if strings.TrimSpace(r.Destination) == "" {
			fail("%s: destination is required", where)
		} else if dir, err := ExpandDestination(r.Destination); err != nil {
			fail("%s: destination: %v", where, err)
		} else {
			r.Dir = dir
			if j, dup := dirs[dir]; dup {
				fail("%s: destination %s duplicates keep.repos[%d]", where, dir, j)
			} else {
				dirs[dir] = i
			}
		}

		if r.Branch != "" && (strings.ContainsAny(r.Branch, " \t\n~^:?*[\\") || strings.HasPrefix(r.Branch, "-")) {
			fail("%s: branch %q is not a valid branch name", where, r.Branch)
		}
		for _, h := range r.PostClone {
			if strings.TrimSpace(h) == "" {
				fail("%s: post_clone contains an empty command", where)
			}
		}
		for _, h := range r.PostPull {
			if strings.TrimSpace(h) == "" {
				fail("%s: post_pull contains an empty command", where)
			}
		}
	}

	// Nested destinations.
	for i := range k.Repos {
		for j := range k.Repos {
			a, b := k.Repos[i].Dir, k.Repos[j].Dir
			if i == j || a == "" || b == "" || a == b {
				continue
			}
			if isInside(b, a) {
				fail("keep.repos[%d]: destination %s is nested inside keep.repos[%d] destination %s", j, b, i, a)
			}
		}
	}

	return errors.Join(errs...)
}

// isInside reports whether child is strictly inside parent.
func isInside(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
