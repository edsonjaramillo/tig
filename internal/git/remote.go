package git

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Remote is a parsed repository location.
type Remote struct {
	// Scheme is the transport: "https", "http", "ssh", "git", "file", ...
	// scp-like URLs (user@host:path) report "ssh".
	Scheme string
	// Host is the lowercased host name without user or port. Empty for file URLs.
	Host string
	// Path is the repository path without leading/trailing slashes or a
	// trailing ".git", e.g. "owner/repo".
	Path string
}

var (
	scpLike     = regexp.MustCompile(`^[^@/:\s]+@[^@/:\s]+:\S+$`)
	segmentChar = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

// IsURL reports whether s is a full URL: it contains "://" or has the
// scp-like form user@host:path.
func IsURL(s string) bool {
	return strings.Contains(s, "://") || scpLike.MatchString(s)
}

// ParseShorthand parses "owner/repo" or "host/owner/repo". The host defaults
// to github.com. The returned Remote has an empty Scheme.
func ParseShorthand(s string) (Remote, error) {
	parts := strings.Split(strings.TrimSuffix(s, ".git"), "/")
	if len(parts) != 2 && len(parts) != 3 {
		return Remote{}, fmt.Errorf("%q is not owner/repo, host/owner/repo, or a full URL", s)
	}
	for _, p := range parts {
		if !segmentChar.MatchString(p) || p == "." || p == ".." {
			return Remote{}, fmt.Errorf("%q is not owner/repo, host/owner/repo, or a full URL", s)
		}
	}
	host := "github.com"
	if len(parts) == 3 {
		host = strings.ToLower(parts[0])
		parts = parts[1:]
	}
	return Remote{Host: host, Path: parts[0] + "/" + parts[1]}, nil
}

// ParseURL parses a full URL ("scheme://..." or scp-like "user@host:path").
// It also accepts "host:path", which git treats as scp-like, so that any
// origin URL git produces can be normalized.
func ParseURL(s string) (Remote, error) {
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return Remote{}, fmt.Errorf("invalid URL %q: %w", s, err)
		}
		r := Remote{
			Scheme: strings.ToLower(u.Scheme),
			Host:   strings.ToLower(u.Hostname()),
			Path:   cleanPath(u.Path),
		}
		if r.Path == "" {
			return Remote{}, fmt.Errorf("URL %q has no repository path", s)
		}
		return r, nil
	}
	colon := strings.Index(s, ":")
	if colon <= 0 || strings.Contains(s[:colon], "/") {
		return Remote{}, fmt.Errorf("%q is not a URL", s)
	}
	host := s[:colon]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	r := Remote{Scheme: "ssh", Host: strings.ToLower(host), Path: cleanPath(s[colon+1:])}
	if r.Host == "" || r.Path == "" {
		return Remote{}, fmt.Errorf("%q is not a URL", s)
	}
	return r, nil
}

func cleanPath(p string) string {
	p = strings.Trim(p, "/")
	p = strings.TrimSuffix(p, ".git")
	return strings.Trim(p, "/")
}

// Name returns the "owner/repo" identifier: the last two path segments.
// It returns "" when the path has fewer than two segments.
func (r Remote) Name() string {
	parts := strings.Split(r.Path, "/")
	if len(parts) < 2 || parts[len(parts)-2] == "" || parts[len(parts)-1] == "" {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// Key returns the protocol-independent identity host/path, lowercased.
// Two remotes with the same Key are the same repository.
func (r Remote) Key() string {
	return strings.ToLower(r.Host + "/" + r.Path)
}

// Transport groups schemes into the protocol a user chooses between:
// "https" (https, http), "ssh" (ssh, scp-like), or the scheme itself.
func (r Remote) Transport() string {
	switch r.Scheme {
	case "http", "https":
		return "https"
	case "ssh", "git+ssh", "ssh+git":
		return "ssh"
	}
	return r.Scheme
}

// HTTPSURL returns https://host/path.git.
func (r Remote) HTTPSURL() string {
	return "https://" + r.Host + "/" + r.Path + ".git"
}

// SSHURL returns git@host:path.git.
func (r Remote) SSHURL() string {
	return "git@" + r.Host + ":" + r.Path + ".git"
}
