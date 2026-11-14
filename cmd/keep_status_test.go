package cmd

import (
	"errors"
	"testing"

	"github.com/edsonjaramillo/tig/internal/keep"
)

func TestFilterStatus(t *testing.T) {
	res := func(name string, wt keep.Worktree, sync keep.SyncState) keep.Result {
		return keep.Result{Repo: keep.KeptRepo{Name: name}, Status: &keep.Status{Worktree: wt, Sync: keep.Sync{State: sync}}}
	}
	all := []keep.Result{
		res("clean", keep.WorktreeClean, keep.SyncUpToDate),
		res("dirty", keep.WorktreeDirty, keep.SyncUpToDate),
		res("behind", keep.WorktreeClean, keep.SyncBehind),
		res("both", keep.WorktreeDirty, keep.SyncDiverged),
		res("missing", keep.WorktreeMissing, keep.SyncNone),
		{Repo: keep.KeptRepo{Name: "failed"}, Status: &keep.Status{}, Err: errors.New("boom")},
	}
	names := func(rs []keep.Result) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.Repo.Name)
		}
		return out
	}
	cases := []struct {
		dirty, outOfSync bool
		want             []string
	}{
		{false, false, []string{"clean", "dirty", "behind", "both", "missing", "failed"}},
		{true, false, []string{"dirty", "both", "failed"}},
		{false, true, []string{"behind", "both", "failed"}},
		{true, true, []string{"dirty", "behind", "both", "failed"}},
	}
	for _, c := range cases {
		got, hidden := filterStatus(all, c.dirty, c.outOfSync)
		if g := names(got); len(g) != len(c.want) || hidden != len(all)-len(c.want) {
			t.Errorf("dirty=%v outOfSync=%v: got %v (hidden %d), want %v", c.dirty, c.outOfSync, g, hidden, c.want)
			continue
		}
		for i := range c.want {
			if names(got)[i] != c.want[i] {
				t.Errorf("dirty=%v outOfSync=%v: got %v, want %v", c.dirty, c.outOfSync, names(got), c.want)
				break
			}
		}
	}
}
