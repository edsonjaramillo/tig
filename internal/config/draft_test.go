package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDraftCommitKeepsModeAndSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "tig.yaml")
	if err := os.MkdirAll(filepath.Dir(real), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte("# old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config", "tig.yaml")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../dotfiles/tig.yaml", link); err != nil {
		t.Fatal(err)
	}

	d, err := NewDraft(link)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Discard()
	if filepath.Dir(d.Path) != filepath.Dir(real) || !strings.HasSuffix(d.Path, ".yaml") {
		t.Errorf("draft path = %s", d.Path)
	}
	if b, _ := d.Read(); string(b) != "# old\n" {
		t.Errorf("draft content = %q", b)
	}
	if changed, _ := d.Changed(); changed {
		t.Error("untouched draft reported as changed")
	}
	if err := os.WriteFile(d.Path, []byte("# new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed, _ := d.Changed(); !changed {
		t.Error("edited draft reported as unchanged")
	}
	if err := d.Commit(); err != nil {
		t.Fatal(err)
	}

	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v", err)
	}
	if b, _ := os.ReadFile(real); string(b) != "# new\n" {
		t.Errorf("real content = %q", b)
	}
	if fi, _ := os.Stat(real); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %o, want 640", fi.Mode().Perm())
	}
	if _, err := os.Stat(d.Path); !os.IsNotExist(err) {
		t.Error("draft left behind after commit")
	}
}

func TestDraftNewFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "tig.yaml")
	d, err := NewDraft(p)
	if err != nil {
		t.Fatal(err)
	}
	if changed, _ := d.Changed(); changed {
		t.Error("empty draft of a missing file reported as changed")
	}
	if err := os.WriteFile(d.Path, []byte("keep: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("new config: %v %v", fi, err)
	}
}

func TestDraftDiscard(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tig.yaml")
	if err := os.WriteFile(p, []byte("# keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := NewDraft(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(d.Path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	d.Discard()
	d.Discard()
	if _, err := os.Stat(d.Path); !os.IsNotExist(err) {
		t.Error("draft not removed")
	}
	if b, _ := os.ReadFile(p); string(b) != "# keep me\n" {
		t.Errorf("original changed: %q", b)
	}
}

func TestDraftRefusesWhenChangedOnDisk(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tig.yaml")
	if err := os.WriteFile(p, []byte("# v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := NewDraft(p)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Discard()
	if err := os.WriteFile(d.Path, []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("# theirs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(); !errors.Is(err, ErrChangedOnDisk) {
		t.Fatalf("err = %v, want ErrChangedOnDisk", err)
	}
	d.Discard()
	if b, _ := os.ReadFile(d.Path); string(b) != "# mine\n" {
		t.Errorf("draft not kept after refused commit: %q", b)
	}
	if b, _ := os.ReadFile(p); string(b) != "# theirs\n" {
		t.Errorf("real file overwritten: %q", b)
	}
}
