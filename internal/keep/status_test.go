package keep

import "testing"

func TestStatusPredicates(t *testing.T) {
	cases := []struct {
		st               *Status
		dirty, outOfSync bool
	}{
		{nil, false, false},
		{&Status{Worktree: WorktreeMissing}, false, false},
		{&Status{Worktree: WorktreeClean, Sync: newSync(0, 0)}, false, false},
		{&Status{Worktree: WorktreeDirty, Sync: newSync(0, 0)}, true, false},
		{&Status{Worktree: WorktreeClean, Sync: newSync(2, 0)}, false, true},
		{&Status{Worktree: WorktreeClean, Sync: newSync(0, 1)}, false, true},
		{&Status{Worktree: WorktreeDirty, Sync: newSync(1, 1)}, true, true},
		{&Status{Worktree: WorktreeClean, Sync: Sync{State: SyncNoUpstream}}, false, false},
		{&Status{Worktree: WorktreeClean, Sync: Sync{State: SyncOffBranch, Branch: "x"}}, false, false},
	}
	for _, c := range cases {
		if got := c.st.IsDirty(); got != c.dirty {
			t.Errorf("%+v IsDirty = %v, want %v", c.st, got, c.dirty)
		}
		if got := c.st.IsOutOfSync(); got != c.outOfSync {
			t.Errorf("%+v IsOutOfSync = %v, want %v", c.st, got, c.outOfSync)
		}
	}
}

func TestSyncString(t *testing.T) {
	cases := map[string]Sync{
		"":                 {},
		"up-to-date":       newSync(0, 0),
		"ahead 2":          newSync(2, 0),
		"behind 1":         newSync(0, 1),
		"diverged":         newSync(1, 3),
		"no-upstream":      {State: SyncNoUpstream},
		"off-branch (dev)": {State: SyncOffBranch, Branch: "dev"},
	}
	for want, s := range cases {
		if got := s.String(); got != want {
			t.Errorf("%+v.String() = %q, want %q", s, got, want)
		}
	}
}
