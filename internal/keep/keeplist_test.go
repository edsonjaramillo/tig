package keep

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Setenv("CODE", "/code")
	tests := []struct {
		name    string
		list    KeepList
		wantErr string // substring; "" means valid
	}{
		{"empty", KeepList{}, ""},
		{"valid", KeepList{Repos: []KeptRepo{
			{Repo: "a/b", Destination: "/x/b"},
			{Repo: "gitlab.com/c/d", Destination: "$CODE/d"},
			{Repo: "git@github.com:e/f.git", Destination: "/x/f"},
		}}, ""},
		{"bad protocol", KeepList{Protocol: "git"}, "keep.protocol"},
		{"missing repo", KeepList{Repos: []KeptRepo{{Destination: "/x"}}}, "repo is required"},
		{"missing destination", KeepList{Repos: []KeptRepo{{Repo: "a/b"}}}, "destination is required"},
		{"bad repo", KeepList{Repos: []KeptRepo{{Repo: "just-a-name", Destination: "/x"}}}, "not owner/repo"},
		{"relative destination", KeepList{Repos: []KeptRepo{{Repo: "a/b", Destination: "code/b"}}}, "not an absolute path"},
		{"undefined env", KeepList{Repos: []KeptRepo{{Repo: "a/b", Destination: "$TIG_NOPE_UNDEFINED/b"}}}, "undefined environment variable"},
		{"duplicate name across hosts", KeepList{Repos: []KeptRepo{
			{Repo: "a/b", Destination: "/x/1"},
			{Repo: "gitlab.com/A/B", Destination: "/x/2"},
		}}, "duplicates"},
		{"duplicate destination", KeepList{Repos: []KeptRepo{
			{Repo: "a/b", Destination: "/x/1"},
			{Repo: "c/d", Destination: "/x/1/"},
		}}, "destination /x/1 duplicates"},
		{"nested destination", KeepList{Repos: []KeptRepo{
			{Repo: "a/b", Destination: "/x"},
			{Repo: "c/d", Destination: "/x/d"},
		}}, "nested"},
		{"sibling prefix is not nested", KeepList{Repos: []KeptRepo{
			{Repo: "a/b", Destination: "/x/b"},
			{Repo: "c/d", Destination: "/x/bb"},
		}}, ""},
		{"empty hook", KeepList{Repos: []KeptRepo{{Repo: "a/b", Destination: "/x", PostClone: []string{" "}}}}, "empty command"},
		{"bad branch", KeepList{Repos: []KeptRepo{{Repo: "a/b", Destination: "/x", Branch: "-x"}}}, "branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.list.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
