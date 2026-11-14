package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validConfig = "# mine\nkeep:\n  repos:\n    - repo: a/b\n      destination: /x/b\n"

// fakeEditor sets $VISUAL to a script that writes each of contents in turn,
// one per editor run, to the file it is given.
func fakeEditor(t *testing.T, contents ...string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nn=$(cat " + dir + "/n 2>/dev/null || echo 0)\nn=$((n+1))\necho $n > " + dir + "/n\ncp " + dir + "/$n \"$1\"\n"
	for i, c := range contents {
		if err := os.WriteFile(filepath.Join(dir, string(rune('1'+i))), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ed := filepath.Join(dir, "editor")
	if err := os.WriteFile(ed, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", ed)
	t.Setenv("EDITOR", "")
}

func fakeTerminal(t *testing.T, yes bool) {
	t.Helper()
	old := isInteractive
	isInteractive = func(io.Reader) bool { return yes }
	t.Cleanup(func() { isInteractive = old })
}

func writeConfig(t *testing.T, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tig.yaml")
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigEditValid(t *testing.T) {
	p := writeConfig(t, "# old\n", 0o640)
	fakeEditor(t, validConfig)
	out, _, err := run(t, "", "--config", p, "config", "edit")
	if err != nil || !strings.Contains(out, "saved "+p) {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(p); string(b) != validConfig {
		t.Errorf("content = %q", b)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %o, want 640", fi.Mode().Perm())
	}
	assertNoDrafts(t, p)
}

func TestConfigEditInvalidWithoutTerminal(t *testing.T) {
	p := writeConfig(t, validConfig, 0o600)
	fakeEditor(t, "kep: {}\n")
	_, errOut, err := run(t, "", "--config", p, "config", "edit")
	if !errors.Is(err, errFailures) {
		t.Fatalf("err = %v, want errFailures", err)
	}
	if !strings.Contains(errOut, `unknown key "kep"`) || !strings.Contains(errOut, "untouched") {
		t.Errorf("stderr:\n%s", errOut)
	}
	if b, _ := os.ReadFile(p); string(b) != validConfig {
		t.Errorf("original changed: %q", b)
	}
	assertNoDrafts(t, p)
}

func TestConfigEditReEdit(t *testing.T) {
	p := writeConfig(t, "", 0o600)
	fakeTerminal(t, true)
	fakeEditor(t, "kep: {}\n", validConfig)
	out, errOut, err := run(t, "\n", "--config", p, "config", "edit")
	if err != nil {
		t.Fatalf("err = %v\n%s", err, errOut)
	}
	if !strings.Contains(errOut, "re-edit? [Y/n]") || !strings.Contains(out, "saved") {
		t.Errorf("stdout:\n%s\nstderr:\n%s", out, errOut)
	}
	if b, _ := os.ReadFile(p); string(b) != validConfig {
		t.Errorf("content = %q", b)
	}
}

func TestConfigEditDeclineReEdit(t *testing.T) {
	p := writeConfig(t, validConfig, 0o600)
	fakeTerminal(t, true)
	fakeEditor(t, "kep: {}\n")
	_, _, err := run(t, "n\n", "--config", p, "config", "edit")
	if !errors.Is(err, errFailures) {
		t.Fatalf("err = %v, want errFailures", err)
	}
	if b, _ := os.ReadFile(p); string(b) != validConfig {
		t.Errorf("original changed: %q", b)
	}
	assertNoDrafts(t, p)
}

func TestConfigEditNoChanges(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tig.yaml")
	t.Setenv("VISUAL", "true")
	out, _, err := run(t, "", "--config", p, "config", "edit")
	if err != nil || !strings.Contains(out, "no changes") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("config created without changes")
	}
	assertNoDrafts(t, p)
}

func TestConfigEditEditorFails(t *testing.T) {
	p := writeConfig(t, validConfig, 0o600)
	t.Setenv("VISUAL", "false")
	if _, _, err := run(t, "", "--config", p, "config", "edit"); err == nil {
		t.Fatal("editor failure not reported")
	}
	assertNoDrafts(t, p)
}

func TestKeepEditIsDeprecatedAlias(t *testing.T) {
	p := writeConfig(t, "", 0o600)
	fakeEditor(t, validConfig)
	_, _, err := run(t, "", "--config", p, "keep", "edit")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != validConfig {
		t.Errorf("content = %q", b)
	}
	help, _, _ := run(t, "", "keep", "--help")
	if strings.Contains(help, "edit") {
		t.Errorf("deprecated alias shown in help:\n%s", help)
	}
}

func TestConfigCheck(t *testing.T) {
	good := writeConfig(t, validConfig, 0o600)
	bad := writeConfig(t, "kep: {}\n", 0o600)
	missing := filepath.Join(t.TempDir(), "nope.yaml")

	if out, _, err := run(t, "", "--config", good, "config", "check"); err != nil || !strings.Contains(out, "is valid") {
		t.Errorf("good: err = %v\n%s", err, out)
	}
	_, errOut, err := run(t, "", "config", "check", good, bad, missing)
	if !errors.Is(err, errFailures) {
		t.Errorf("bad files: err = %v, want errFailures", err)
	}
	if !strings.Contains(errOut, `unknown key "kep"`) || !strings.Contains(errOut, "not found") {
		t.Errorf("stderr:\n%s", errOut)
	}

	loose := writeConfig(t, validConfig, 0o666)
	if err := os.Chmod(loose, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, errOut, err := run(t, "", "config", "check", loose); err != nil || !strings.Contains(errOut, "warning:") {
		t.Errorf("loose permissions: err = %v\n%s", err, errOut)
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("TIG_CONFIG", "/env/tig.yaml")
	out, errOut, err := run(t, "", "config", "path", "-v")
	if err != nil || out != "/env/tig.yaml\n" || !strings.Contains(errOut, "from $TIG_CONFIG; does not exist") {
		t.Errorf("err = %v, stdout %q, stderr %q", err, out, errOut)
	}
}

func TestEmptyKeepList(t *testing.T) {
	p := writeConfig(t, "keep: {}\n", 0o600)
	for _, cmd := range []string{"list", "status", "clone", "pull"} {
		_, errOut, err := run(t, "", "--config", p, "keep", cmd)
		if err != nil || !strings.Contains(errOut, "the keep list is empty; add repos with `tig config edit`") {
			t.Errorf("%s: err = %v\n%s", cmd, err, errOut)
		}
	}
}

func assertNoDrafts(t *testing.T, config string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Dir(config))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("draft left behind: %s", e.Name())
		}
	}
}

// run runs the tig command tree with argv, feeding stdin.
func run(t *testing.T, stdin string, argv ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd("test", argv)
	var out, errOut strings.Builder
	root.SetIn(strings.NewReader(stdin))
	root.SetOut(&out)
	root.SetErr(&errOut)
	err = root.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}
