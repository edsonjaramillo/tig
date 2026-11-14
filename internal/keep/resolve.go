package keep

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edsonjaramillo/tig/internal/git"
)

// resolveRepo derives Name, CloneURL, and Remote from the repo field.
func (r *KeptRepo) resolveRepo(protocol string) error {
	if git.IsURL(r.Repo) {
		remote, err := git.ParseURL(r.Repo)
		if err != nil {
			return err
		}
		name := remote.Name()
		if name == "" {
			return fmt.Errorf("cannot derive owner/repo from %q", r.Repo)
		}
		r.Remote, r.Name, r.CloneURL = remote, name, r.Repo
		return nil
	}
	remote, err := git.ParseShorthand(r.Repo)
	if err != nil {
		return err
	}
	if protocol == ProtocolSSH {
		remote.Scheme = "ssh"
		r.CloneURL = remote.SSHURL()
	} else {
		remote.Scheme = "https"
		r.CloneURL = remote.HTTPSURL()
	}
	r.Remote, r.Name = remote, remote.Name()
	return nil
}

// ExpandDestination expands a leading ~ and environment variables, and
// requires the result to be absolute. Undefined variables are an error.
func ExpandDestination(dest string) (string, error) {
	var missing []string
	expanded := os.Expand(dest, func(key string) string {
		v, ok := os.LookupEnv(key)
		if !ok {
			missing = append(missing, key)
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("undefined environment variable $%s", strings.Join(missing, ", $"))
	}

	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expanding ~: %w", err)
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	} else if strings.HasPrefix(expanded, "~") {
		return "", errors.New("~user paths are not supported")
	}

	if !filepath.IsAbs(expanded) {
		return "", fmt.Errorf("%q is not an absolute path", dest)
	}
	return filepath.Clean(expanded), nil
}
