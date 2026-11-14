package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Draft is a temporary copy of the tig config that is edited instead of the
// real file, so an invalid config never reaches disk (like visudo or
// crontab -e). Commit moves the user's bytes into place unchanged; tig still
// never re-serializes the file (ADR-0003).
type Draft struct {
	// Path is the temporary file to edit. It lives next to the real file and
	// ends in .yaml so editors pick the right syntax.
	Path string

	target  string      // the real file, with symlinks resolved
	orig    []byte      // target's content when the draft was made
	existed bool        // whether target existed when the draft was made
	mode    fs.FileMode // permissions to give the target
	done    bool
}

// NewDraft copies the config at path (or nothing, if it does not exist) to
// a new temporary file. If path is a symlink, as with dotfile managers, the
// draft is made next to, and later replaces, the file the link points to, so
// the link survives. The draft keeps the original's permissions; a new config
// gets 0600.
func NewDraft(path string) (*Draft, error) {
	target, err := resolveLinks(path)
	if err != nil {
		return nil, err
	}
	d := &Draft{target: target, mode: 0o600}
	if fi, err := os.Stat(target); err == nil {
		d.existed, d.mode = true, fi.Mode().Perm()
		if d.orig, err = os.ReadFile(target); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*.yaml")
	if err != nil {
		return nil, err
	}
	d.Path = f.Name()
	_, werr := f.Write(d.orig)
	cerr := f.Close()
	if err := errors.Join(werr, cerr, os.Chmod(d.Path, d.mode)); err != nil {
		_ = os.Remove(d.Path)
		return nil, err
	}
	return d, nil
}

// Read returns the draft's current content.
func (d *Draft) Read() ([]byte, error) { return os.ReadFile(d.Path) }

// Changed reports whether the draft differs from the original.
func (d *Draft) Changed() (bool, error) {
	data, err := d.Read()
	if err != nil {
		return false, err
	}
	return !bytes.Equal(data, d.orig), nil
}

// ErrChangedOnDisk is returned by Commit when the real config changed while
// the draft was being edited.
var ErrChangedOnDisk = errors.New("the tig config changed on disk while you were editing")

// Commit atomically replaces the real config with the draft. It refuses, and
// keeps the draft, if the real config changed since the draft was made.
func (d *Draft) Commit() error {
	now, err := os.ReadFile(d.target)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if exists != d.existed || !bytes.Equal(now, d.orig) {
		d.done = true // keep the draft so the edits are not lost
		return fmt.Errorf("%w; your edits are in %s", ErrChangedOnDisk, d.Path)
	}
	f, err := os.OpenFile(d.Path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	serr := f.Sync()
	if err := errors.Join(serr, f.Close()); err != nil {
		return err
	}
	// Editors that save by renaming a new file over the old one may not keep
	// the draft's permissions.
	if err := os.Chmod(d.Path, d.mode); err != nil {
		return err
	}
	if err := os.Rename(d.Path, d.target); err != nil {
		return err
	}
	d.done = true
	return nil
}

// Discard removes the draft unless it was committed or kept. It is safe to
// call more than once, so it can be deferred.
func (d *Draft) Discard() {
	if !d.done {
		_ = os.Remove(d.Path)
		d.done = true
	}
}

// resolveLinks follows path through any symlinks to the file they finally
// point to, which need not exist.
func resolveLinks(path string) (string, error) {
	for range 40 {
		fi, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&fs.ModeSymlink == 0 {
			return path, nil
		}
		link, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		path = link
	}
	return "", fmt.Errorf("%s: too many levels of symbolic links", path)
}
