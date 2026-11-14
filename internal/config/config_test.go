package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		repos   int
	}{
		{"empty file", "", "", 0},
		{"comments only", "# nothing\n", "", 0},
		{"full", `
keep:
  protocol: ssh
  repos:
    - repo: edsonjaramillo/tig
      destination: /code/tig
      branch: main
      post_clone: [task build]
      post_pull: [go mod download]
    - repo: git@github.com:a/dotfiles.git
      destination: /home/a/.dotfiles
`, "", 2},
		{"top-level typo", "kep:\n  repos: []\n", `unknown key "kep" at the top level (known keys: keep)`, 0},
		{"nested unknown key", "keep:\n  repos:\n    - repo: a/b\n      destination: /x\n      name: b\n", `unknown key "name" in a keep.repos entry`, 0},
		{"resolved field not settable", "keep:\n  repos:\n    - repo: a/b\n      destination: /x\n      dir: /y\n", `unknown key "dir"`, 0},
		{"repos not a list", "keep:\n  repos:\n    repo: a/b\n    destination: /x\n", `line 3: keep.repos must be a list of repos, not a mapping (start each repo with "- "`, 0},
		{"hooks not a list", "keep:\n  repos:\n    - repo: a/b\n      destination: /x\n      post_pull: {a: b}\n", "must be a list of commands, not a mapping", 0},
		{"keep not a mapping", "keep: [1]\n", "keep must be a mapping with protocol and repos, not a list", 0},
		{"invalid", "keep:\n  repos:\n    - repo: a/b\n", "destination is required", 0},
		{"second document", "keep: {}\n---\nkep: {}\n", `line 2: only one YAML document is allowed`, 0},
		{"trailing document separator", "keep: {}\n---\n", `only one YAML document is allowed`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse([]byte(tt.yaml), "tig.yaml")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Keep) != tt.repos {
				t.Fatalf("repos = %d, want %d", len(cfg.Keep), tt.repos)
			}
		})
	}
}

func TestParseResolvesSSH(t *testing.T) {
	cfg, err := Parse([]byte("keep:\n  protocol: ssh\n  repos:\n    - repo: a/b\n      destination: /x\n"), "t")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Keep[0].CloneURL; got != "git@github.com:a/b.git" {
		t.Errorf("CloneURL = %q", got)
	}
}

func TestPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	t.Setenv("TIG_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if p, _ := Path(""); p != filepath.Join(home, ".config", "tig.yaml") {
		t.Errorf("default = %q", p)
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	if p, _ := Path(""); p != filepath.Join(home, ".config", "tig.yaml") {
		t.Errorf("relative XDG should be ignored, got %q", p)
	}
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if p, _ := Path(""); p != "/xdg/tig.yaml" {
		t.Errorf("xdg = %q", p)
	}
	t.Setenv("TIG_CONFIG", "/env/tig.yaml")
	if p, _ := Path(""); p != "/env/tig.yaml" {
		t.Errorf("env = %q", p)
	}
	if p, _ := Path("/flag.yaml"); p != "/flag.yaml" {
		t.Errorf("flag = %q", p)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestPermissionWarning(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tig.yaml")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if w := PermissionWarning(p); w != "" {
		t.Errorf("0600 warned: %s", w)
	}
	if err := os.Chmod(p, 0o664); err != nil {
		t.Fatal(err)
	}
	if w := PermissionWarning(p); w == "" {
		t.Error("0664 did not warn")
	}
}

func TestAppendStarterKeep(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	p := filepath.Join(dir, "tig.yaml")

	if err := AppendStarterKeep(p); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o", fi.Mode().Perm())
	}
	if _, err := Load(p); err != nil {
		t.Fatalf("starter is not a valid config: %v", err)
	}
	if err := AppendStarterKeep(p); err == nil || !strings.Contains(err.Error(), "already has a keep section") {
		t.Fatalf("second init err = %v", err)
	}

	// Appending preserves existing text verbatim.
	p2 := filepath.Join(t.TempDir(), "tig.yaml")
	existing := "# my comment\n# other: settings"
	if err := os.WriteFile(p2, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendStarterKeep(p2); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p2)
	if !strings.HasPrefix(string(b), existing+"\n\n") {
		t.Errorf("existing content not preserved:\n%s", b)
	}
	if _, err := Load(p2); err != nil {
		t.Fatalf("appended config invalid: %v", err)
	}
}
