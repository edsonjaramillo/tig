package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGH puts a gh script on PATH that acts as account alice on github.com.
// The repo exists when exists is true. It returns the path of a log holding
// one line of arguments per gh call.
func fakeGH(t *testing.T, exists bool) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	existsCode := "1"
	if exists {
		existsCode = "0"
	}
	script := `#!/bin/sh
echo "$*" >> ` + log + `
case "$1 $2" in
"api user") echo "alice https://github.com/alice" ;;
"api repos/"*) [ ` + existsCode + ` = 0 ] && exit 0; echo "gh: Not Found (HTTP 404)" >&2; exit 1 ;;
"repo create") exit 0 ;;
*) echo "unexpected: $*" >&2; exit 9 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// gitRepo creates a git repository named name and changes into it.
func gitRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Chdir(dir)
	return dir
}

func origin(t *testing.T, dir string) string {
	t.Helper()
	out, _ := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	return strings.TrimSpace(string(out))
}

func readLog(t *testing.T, p string) string {
	t.Helper()
	b, _ := os.ReadFile(p)
	return string(b)
}

func TestRepoCreatePublicDefaultName(t *testing.T) {
	log := fakeGH(t, false)
	dir := gitRepo(t, "proj")
	out, _, err := run(t, "", "repo", "create")
	if err != nil {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if !strings.Contains(readLog(t, log), "repo create proj --public") {
		t.Errorf("gh calls:\n%s", readLog(t, log))
	}
	if got := origin(t, dir); got != "https://github.com/alice/proj.git" {
		t.Errorf("origin = %q", got)
	}
	if !strings.Contains(out, "created public repo alice/proj") {
		t.Errorf("out = %q", out)
	}
}

func TestRepoCreatePrivateNamed(t *testing.T) {
	log := fakeGH(t, false)
	dir := gitRepo(t, "proj")
	if _, _, err := run(t, "", "repo", "create", "other", "--private"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readLog(t, log), "repo create other --private") {
		t.Errorf("gh calls:\n%s", readLog(t, log))
	}
	if got := origin(t, dir); got != "https://github.com/alice/other.git" {
		t.Errorf("origin = %q", got)
	}
}

func TestRepoCreateRefuses(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
		setup  func(t *testing.T, dir string)
		args   []string
		want   string
	}{
		{name: "exists", exists: true, want: "alice/proj already exists"},
		{name: "owner in name", args: []string{"org/x"}, want: "invalid repo name"},
		{
			name: "origin set",
			setup: func(t *testing.T, dir string) {
				if err := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://x/y.git").Run(); err != nil {
					t.Fatal(err)
				}
			},
			want: "origin is already set",
		},
		{
			name: "not repo root",
			setup: func(t *testing.T, dir string) {
				sub := filepath.Join(dir, "sub")
				if err := os.Mkdir(sub, 0o700); err != nil {
					t.Fatal(err)
				}
				t.Chdir(sub)
			},
			want: "no .git here",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := fakeGH(t, tt.exists)
			dir := gitRepo(t, "proj")
			if tt.setup != nil {
				tt.setup(t, dir)
			}
			_, _, err := run(t, "", append([]string{"repo", "create"}, tt.args...)...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if strings.Contains(readLog(t, log), "repo create") {
				t.Errorf("repo was created:\n%s", readLog(t, log))
			}
		})
	}
}
