package keep

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		repo, protocol string
		name, url      string
	}{
		{"edsonjaramillo/tig", "", "edsonjaramillo/tig", "https://github.com/edsonjaramillo/tig.git"},
		{"edsonjaramillo/tig", "https", "edsonjaramillo/tig", "https://github.com/edsonjaramillo/tig.git"},
		{"edsonjaramillo/tig", "ssh", "edsonjaramillo/tig", "git@github.com:edsonjaramillo/tig.git"},
		{"gitlab.com/someone/thing", "ssh", "someone/thing", "git@gitlab.com:someone/thing.git"},
		{"git@github.com:a/dotfiles.git", "https", "a/dotfiles", "git@github.com:a/dotfiles.git"},
		{"https://example.com/a/b", "ssh", "a/b", "https://example.com/a/b"},
	}
	for _, tt := range tests {
		var r KeptRepo
		if err := r.resolveRepo(tt.repo, tt.protocol); err != nil {
			t.Errorf("resolveRepo(%q, %q): %v", tt.repo, tt.protocol, err)
			continue
		}
		if r.Name != tt.name || r.CloneURL != tt.url {
			t.Errorf("resolveRepo(%q, %q) = %q %q, want %q %q", tt.repo, tt.protocol, r.Name, r.CloneURL, tt.name, tt.url)
		}
	}
}

func TestExpandDestination(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("TIG_TEST_DIR", "/srv")
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{"~", home, false},
		{"~/code/tig", filepath.Join(home, "code/tig"), false},
		{"$TIG_TEST_DIR/x", "/srv/x", false},
		{"${TIG_TEST_DIR}/x/../y", "/srv/y", false},
		{"/abs/", "/abs", false},
		{"rel/path", "", true},
		{"~other/x", "", true},
		{"$TIG_TEST_UNDEFINED_VAR/x", "", true},
	}
	for _, tt := range tests {
		got, err := ExpandDestination(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ExpandDestination(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ExpandDestination(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
