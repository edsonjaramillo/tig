package config

import (
	"strings"
	"testing"
)

// TestHumanizeCoversSections decodes one bad document per section and checks
// that yaml.v3's Go-type wording never reaches the user. It fails if a type
// rename or a yaml.v3 upgrade breaks the lookup in sections.
func TestHumanizeCoversSections(t *testing.T) {
	cases := []struct {
		yaml string
		want string
	}{
		{"kep: {}\n", "at the top level (known keys: keep)"},
		{"[1]\n", "must be a mapping"},
		{"keep:\n  protcol: ssh\n", "in keep (known keys: protocol, repos)"},
		{"keep: [1]\n", "keep must be a mapping with protocol and repos"},
		{"keep:\n  repos:\n    - nme: x\n", "in a keep.repos entry (known keys: repo, destination, branch, post_clone, post_pull)"},
		{"keep:\n  repos: {}\n", "keep.repos must be a list of repos"},
		{"keep:\n  repos:\n    - post_clone: {a: b}\n", "post_clone/post_pull must be a list of commands"},
		{"keep:\n  repos:\n    - repo: [a]\n", "must be a single value"},
	}
	if got := len(sections); got != 6 {
		t.Fatalf("sections has %d entries; add a case here for each new one", got)
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.yaml), "tig.yaml")
		if err == nil {
			t.Errorf("%q: no error", c.yaml)
			continue
		}
		msg := err.Error()
		if strings.Contains(msg, "not found in type") || strings.Contains(msg, "cannot unmarshal") {
			t.Errorf("%q: raw yaml.v3 error leaked: %s", c.yaml, msg)
		}
		if !strings.Contains(msg, c.want) {
			t.Errorf("%q: error %q, want containing %q", c.yaml, msg, c.want)
		}
	}
}
