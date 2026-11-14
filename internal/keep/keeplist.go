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

// ErrEmpty is returned when the keep list declares no kept repos.
var ErrEmpty = errors.New("the keep list is empty; add repos with `tig config edit`")

// KeepList is the `keep:` section of the tig config, as written by the user.
// Resolve turns it into kept repos.
type KeepList struct {
	// Protocol applies to shorthand repos: "https" (default) or "ssh".
	Protocol string         `yaml:"protocol"`
	Repos    []keptRepoSpec `yaml:"repos"`
}

// keptRepoSpec is one entry of the keep list as written in the tig config.
type keptRepoSpec struct {
	Repo        string   `yaml:"repo"`
	Destination string   `yaml:"destination"`
	Branch      string   `yaml:"branch"`
	PostClone   []string `yaml:"post_clone"`
	PostPull    []string `yaml:"post_pull"`
}

// KeptRepo is one resolved entry in the keep list.
type KeptRepo struct {
	Name      string     // repo name: owner/repo
	CloneURL  string     // URL passed to git clone
	Remote    git.Remote // parsed clone URL
	Dir       string     // destination: expanded and absolute
	Branch    string     // declared default branch; "" means the remote's HEAD
	PostClone []string
	PostPull  []string
}

// Resolve checks the keep list and returns its kept repos. All problems are
// reported together.
//
// Destinations are compared case-insensitively on every platform, so the
// result does not depend on the filesystem of the machine running it; two
// destinations differing only in case are a mistake either way.
func (k KeepList) Resolve() ([]KeptRepo, error) {
	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	switch k.Protocol {
	case "", ProtocolHTTPS, ProtocolSSH:
	default:
		fail("keep.protocol: %q is not https or ssh", k.Protocol)
	}

	repos := make([]KeptRepo, len(k.Repos))
	names := map[string]int{}
	dirs := map[string]int{}
	for i, s := range k.Repos {
		r := &repos[i]
		r.Branch, r.PostClone, r.PostPull = s.Branch, s.PostClone, s.PostPull
		where := fmt.Sprintf("keep.repos[%d]", i)
		if s.Repo != "" {
			where += " (" + s.Repo + ")"
		}

		if strings.TrimSpace(s.Repo) == "" {
			fail("%s: repo is required", where)
		} else if err := r.resolveRepo(s.Repo, k.Protocol); err != nil {
			fail("%s: %v", where, err)
		} else {
			key := strings.ToLower(r.Name)
			if j, dup := names[key]; dup {
				fail("%s: repo name %s duplicates keep.repos[%d]", where, r.Name, j)
			} else {
				names[key] = i
			}
		}

		if strings.TrimSpace(s.Destination) == "" {
			fail("%s: destination is required", where)
		} else if dir, err := ExpandDestination(s.Destination); err != nil {
			fail("%s: destination: %v", where, err)
		} else {
			r.Dir = dir
			key := strings.ToLower(dir)
			if j, dup := dirs[key]; dup {
				fail("%s: destination %s duplicates keep.repos[%d]", where, dir, j)
			} else {
				dirs[key] = i
			}
		}

		if s.Branch != "" && (strings.ContainsAny(s.Branch, " \t\n~^:?*[\\") || strings.HasPrefix(s.Branch, "-")) {
			fail("%s: branch %q is not a valid branch name", where, s.Branch)
		}
		for _, h := range s.PostClone {
			if strings.TrimSpace(h) == "" {
				fail("%s: post_clone contains an empty command", where)
			}
		}
		for _, h := range s.PostPull {
			if strings.TrimSpace(h) == "" {
				fail("%s: post_pull contains an empty command", where)
			}
		}
	}

	// Nested destinations. O(n²) is fine at realistic keep list sizes.
	for i := range repos {
		for j := range repos {
			a, b := strings.ToLower(repos[i].Dir), strings.ToLower(repos[j].Dir)
			if i == j || a == "" || b == "" || a == b {
				continue
			}
			if isInside(b, a) {
				fail("keep.repos[%d]: destination %s is nested inside keep.repos[%d] destination %s", j, repos[j].Dir, i, repos[i].Dir)
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return repos, nil
}

// isInside reports whether child is strictly inside parent.
func isInside(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
