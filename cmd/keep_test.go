package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a tig config with two kept repos backed by local bare remotes:
// me/proj (checked out) and me/other (missing).
type fixture struct {
	config string
	proj   string // destination of me/proj
	other  string // destination of me/other
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	f := fixture{
		config: filepath.Join(root, "tig.yaml"),
		proj:   filepath.Join(root, "code", "proj"),
		other:  filepath.Join(root, "code", "other"),
	}
	projURL := newBareRemote(t, root, "me", "proj")
	otherURL := newBareRemote(t, root, "me", "other")
	gitT(t, root, "clone", "-q", projURL, f.proj)
	cfg := fmt.Sprintf(`keep:
  repos:
    - repo: %s
      destination: %s
      post_clone: ["touch clone.out"]
      post_pull: ["touch pull.out"]
    - repo: %s
      destination: %s
`, projURL, f.proj, otherURL, f.other)
	if err := os.WriteFile(f.config, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

// tig runs the tig command tree with args against the fixture's config.
func (f fixture) tig(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var argv []string
	if len(args) == 0 || args[0] != "__complete" {
		argv = append([]string{"--config", f.config}, args...)
	} else {
		argv = append([]string{"__complete", "--config", f.config}, args[1:]...)
	}
	root := newRootCmd("test", argv)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	err = root.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

func TestUnknownNamesAreErrors(t *testing.T) {
	f := newFixture(t)
	for _, args := range [][]string{
		{"keep", "list", "nope"},
		{"keep", "status", "proj", "nope"},
		{"keep", "hooks", "proj", "nope"},
		{"keep", "run", "proj", "nope", "--", "touch", "ran.out"},
	} {
		_, _, err := f.tig(t, args...)
		if err == nil || !strings.Contains(err.Error(), `"nope" matches no kept repo`) {
			t.Errorf("%v: err = %v", args, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.proj, "ran.out")); !os.IsNotExist(err) {
		t.Error("run executed despite an unknown name")
	}
	if _, err := os.Stat(filepath.Join(f.proj, "clone.out")); !os.IsNotExist(err) {
		t.Error("hooks executed despite an unknown name")
	}
}

func TestCompletion(t *testing.T) {
	f := newFixture(t)
	complete := func(args ...string) string {
		t.Helper()
		out, _, err := f.tig(t, append([]string{"__complete"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	out := complete("keep", "pull", "")
	for _, want := range []string{"me/proj\t", "me/other\t", ":4\n"} { // 4 = NoFileComp
		if !strings.Contains(out, want) {
			t.Errorf("pull completion missing %q:\n%s", want, out)
		}
	}
	if out := complete("keep", "pull", "proj", ""); strings.Contains(out, "me/proj") || !strings.Contains(out, "me/other") {
		t.Errorf("already given names should be left out:\n%s", out)
	}
	if out := complete("keep", "status", "ot"); !strings.Contains(out, "other\t") || strings.Contains(out, "me/proj") {
		t.Errorf("short-name completion:\n%s", out)
	}
	if out := complete("keep", "run", "proj", "--", ""); strings.Contains(out, "me/other") || !strings.Contains(out, ":0\n") {
		t.Errorf("after -- completion should fall back to default:\n%s", out)
	}
}

func TestHooksPostPull(t *testing.T) {
	f := newFixture(t)
	out, _, err := f.tig(t, "keep", "hooks", "--post-pull", "proj")
	if err != nil {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(f.proj, "pull.out")); err != nil {
		t.Error("post_pull hook did not run")
	}
	if _, err := os.Stat(filepath.Join(f.proj, "clone.out")); !os.IsNotExist(err) {
		t.Error("post_clone hook ran with --post-pull")
	}
}

func TestRunShellAndExitCode(t *testing.T) {
	f := newFixture(t)

	out, _, err := f.tig(t, "keep", "run", "--shell", "proj", "--", `echo "$TIG_REPO_NAME" | tr a-z A-Z`)
	if err != nil || !strings.Contains(out, "│ ME/PROJ") {
		t.Fatalf("run --shell: err = %v\n%s", err, out)
	}

	// One checkout: tig exits with the command's exit code.
	_, _, err = f.tig(t, "keep", "run", "proj", "--", "sh", "-c", "exit 3")
	var ee exitError
	if !errors.As(err, &ee) || ee.code != 3 {
		t.Fatalf("single repo exit = %v, want exit 3", err)
	}

	// Several repos (me/other is skipped as missing): plain failure.
	_, _, err = f.tig(t, "keep", "run", "--", "sh", "-c", "exit 3")
	if !errors.Is(err, errFailures) {
		t.Fatalf("all repos exit = %v, want errFailures", err)
	}
}

func isolateGit(t *testing.T) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

func gitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// newBareRemote creates <root>/remotes/<owner>/<repo>.git with one commit on
// main and returns its file:// URL.
func newBareRemote(t *testing.T, root, owner, repo string) string {
	t.Helper()
	bare := filepath.Join(root, "remotes", owner, repo+".git")
	work := filepath.Join(root, "work-"+repo)
	gitT(t, root, "init", "-q", "--bare", "-b", "main", bare)
	gitT(t, root, "init", "-q", "-b", "main", work)
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, work, "add", ".")
	gitT(t, work, "commit", "-q", "-m", "init")
	gitT(t, work, "remote", "add", "origin", bare)
	gitT(t, work, "push", "-q", "origin", "main")
	return "file://" + bare
}
