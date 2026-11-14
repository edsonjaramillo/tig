// Package config locates, loads, and validates the tig config: the single
// YAML file shared by all tig features (ADR-0003).
//
// The file is decoded strictly into one root Config struct, so unknown keys at
// any level (including top-level typos like `kep:`) are errors. Each feature
// registers its top-level key in Config. tig never re-serializes the file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/edsonjaramillo/tig/internal/keep"
)

// file is the root of the tig config file. It is the one struct the file is
// strictly decoded into; every feature registers its top-level key here.
type file struct {
	Keep keep.KeepList `yaml:"keep"`
}

// Config is the resolved tig config.
type Config struct {
	// Keep holds the kept repos declared in the keep list, in file order.
	Keep []keep.KeptRepo
}

// ErrNotFound is returned by Load when the config file does not exist.
var ErrNotFound = errors.New("tig config not found")

// Path resolves the config file location: flag, then $TIG_CONFIG, then
// $XDG_CONFIG_HOME/tig.yaml, then ~/.config/tig.yaml (on every platform).
func Path(flag string) (string, error) {
	p, _, err := Locate(flag)
	return p, err
}

// Locate is Path that also reports which rule chose the path.
func Locate(flag string) (path, source string, err error) {
	if flag != "" {
		p, err := filepath.Abs(flag)
		return p, "--config", err
	}
	if p := os.Getenv("TIG_CONFIG"); p != "" {
		p, err := filepath.Abs(p)
		return p, "$TIG_CONFIG", err
	}
	x := os.Getenv("XDG_CONFIG_HOME")
	if x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "tig.yaml"), "$XDG_CONFIG_HOME", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("locating tig config: %w", err)
	}
	source = "default"
	if x != "" {
		source = "default; $XDG_CONFIG_HOME is ignored because it is not absolute"
	}
	return filepath.Join(home, ".config", "tig.yaml"), source, nil
}

// Load reads, strictly decodes, and validates the config at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w at %s (create one with `tig keep init` or `tig config edit`)", ErrNotFound, path)
	}
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

// Parse strictly decodes, validates, and resolves config data. name is used
// in errors.
func Parse(data []byte, name string) (*Config, error) {
	var f file
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("%s:\n%w", name, humanize(err))
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("%s:\n%w", name, err)
		}
		return nil, fmt.Errorf("%s:\nline %d: only one YAML document is allowed (remove the extra \"---\")", name, extra.Line)
	}
	repos, err := f.Keep.Resolve()
	if err != nil {
		return nil, fmt.Errorf("%s: invalid config:\n%w", name, err)
	}
	return &Config{Keep: repos}, nil
}

// PermissionWarning returns a warning when the config file is group- or
// world-writable (its hooks run arbitrary shell), or "" otherwise.
func PermissionWarning(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Sprintf("%s is group- or world-writable (mode %04o); its hooks run arbitrary shell commands. Consider: chmod 600 %s",
			path, fi.Mode().Perm(), path)
	}
	return ""
}

// HasTopLevelKey reports whether the YAML data has key at its top level.
func HasTopLevelKey(data []byte, key string) (bool, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	if len(doc.Content) == 0 {
		return false, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false, nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return true, nil
		}
	}
	return false, nil
}

// StarterKeep is the commented starter block appended by `tig keep init`.
const StarterKeep = `# Repositories tig keeps on this machine. Run ` + "`tig keep clone`" + ` after editing.
keep:
  # protocol: https # https | ssh; default https; applies to shorthand repos
  repos:
    # - repo: owner/repo # owner/repo | host/owner/repo | full URL
    #   destination: ~/code/repo # required
    #   branch: main # optional; default: the remote's HEAD branch
    #   post_clone: [task build] # optional; run after a fresh clone
    #   post_pull: [go mod download] # optional; run after a pull that moved HEAD
`

// AppendStarterKeep appends StarterKeep to the config at path, creating the
// file (mode 0600) and its directory if needed. It refuses when a top-level
// `keep` key already exists.
func AppendStarterKeep(path string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	has, err := HasTopLevelKey(data, "keep")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if has {
		return fmt.Errorf("%s already has a keep section", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	text := StarterKeep
	if len(data) > 0 {
		text = "\n" + text
		if data[len(data)-1] != '\n' {
			text = "\n" + text
		}
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
