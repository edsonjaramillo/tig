package keep

import (
	"strings"
	"testing"
)

func TestKeepListResolve(t *testing.T) {
	t.Setenv("CODE", "/code")
	tests := []struct {
		name    string
		list    KeepList
		wantErr string // substring; "" means valid
	}{
		{"empty", KeepList{}, ""},
		{"valid", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x/b"},
			{Repo: "gitlab.com/c/d", Destination: "$CODE/d"},
			{Repo: "git@github.com:e/f.git", Destination: "/x/f"},
		}}, ""},
		{"bad protocol", KeepList{Protocol: "git"}, "keep.protocol"},
		{"missing repo", KeepList{Repos: []keptRepoSpec{{Destination: "/x"}}}, "repo is required"},
		{"missing destination", KeepList{Repos: []keptRepoSpec{{Repo: "a/b"}}}, "destination is required"},
		{"bad repo", KeepList{Repos: []keptRepoSpec{{Repo: "just-a-name", Destination: "/x"}}}, "not owner/repo"},
		{"relative destination", KeepList{Repos: []keptRepoSpec{{Repo: "a/b", Destination: "code/b"}}}, "not an absolute path"},
		{"undefined env", KeepList{Repos: []keptRepoSpec{{Repo: "a/b", Destination: "$TIG_NOPE_UNDEFINED/b"}}}, "undefined environment variable"},
		{"duplicate name across hosts", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x/1"},
			{Repo: "gitlab.com/A/B", Destination: "/x/2"},
		}}, "duplicates"},
		{"duplicate destination", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x/1"},
			{Repo: "c/d", Destination: "/x/1/"},
		}}, "destination /x/1 duplicates"},
		{"nested destination", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x"},
			{Repo: "c/d", Destination: "/x/d"},
		}}, "nested"},
		{"duplicate destination differing in case", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x/Tig"},
			{Repo: "c/d", Destination: "/x/tig"},
		}}, "destination /x/tig duplicates"},
		{"nested destination differing in case", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x/Code"},
			{Repo: "c/d", Destination: "/x/code/d"},
		}}, "nested"},
		{"sibling prefix is not nested", KeepList{Repos: []keptRepoSpec{
			{Repo: "a/b", Destination: "/x/b"},
			{Repo: "c/d", Destination: "/x/bb"},
		}}, ""},
		{"empty hook", KeepList{Repos: []keptRepoSpec{{Repo: "a/b", Destination: "/x", PostClone: []string{" "}}}}, "empty command"},
		{"bad branch", KeepList{Repos: []keptRepoSpec{{Repo: "a/b", Destination: "/x", Branch: "-x"}}}, "branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.list.Resolve()
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
