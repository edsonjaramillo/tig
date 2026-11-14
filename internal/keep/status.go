package keep

import "fmt"

// Worktree is the state of a checkout's working tree.
type Worktree string

// Worktree states.
const (
	WorktreeMissing Worktree = "missing"
	WorktreeClean   Worktree = "clean"
	WorktreeDirty   Worktree = "dirty"
)

// SyncState classifies a checkout against origin/<default branch>.
type SyncState int

// Sync states.
const (
	// SyncNone means sync was not determined, e.g. the checkout is missing.
	SyncNone SyncState = iota
	SyncUpToDate
	SyncAhead
	SyncBehind
	SyncDiverged
	// SyncNoUpstream means the default branch or origin/<default branch>
	// is unknown locally.
	SyncNoUpstream
	// SyncOffBranch means the checkout is not on its default branch.
	SyncOffBranch
)

// Sync is a checkout's position relative to origin/<default branch>.
type Sync struct {
	State  SyncState
	Ahead  int
	Behind int
	// Branch is the checked-out branch, set for SyncOffBranch.
	Branch string
}

// newSync classifies ahead/behind counts.
func newSync(ahead, behind int) Sync {
	s := Sync{Ahead: ahead, Behind: behind}
	switch {
	case ahead > 0 && behind > 0:
		s.State = SyncDiverged
	case ahead > 0:
		s.State = SyncAhead
	case behind > 0:
		s.State = SyncBehind
	default:
		s.State = SyncUpToDate
	}
	return s
}

// String renders the sync state for display, e.g. "behind 2".
func (s Sync) String() string {
	switch s.State {
	case SyncUpToDate:
		return "up-to-date"
	case SyncAhead:
		return fmt.Sprintf("ahead %d", s.Ahead)
	case SyncBehind:
		return fmt.Sprintf("behind %d", s.Behind)
	case SyncDiverged:
		return "diverged"
	case SyncNoUpstream:
		return "no-upstream"
	case SyncOffBranch:
		return "off-branch (" + s.Branch + ")"
	}
	return ""
}

// OutOfSync reports whether the default branch differs from origin: ahead,
// behind, or diverged.
func (s Sync) OutOfSync() bool {
	switch s.State {
	case SyncAhead, SyncBehind, SyncDiverged:
		return true
	}
	return false
}

// Status is the state of one checkout as reported by `tig keep status`.
type Status struct {
	Branch   string
	Worktree Worktree
	Sync     Sync
	Warning  string
}

// IsDirty reports whether the checkout is dirty.
func (s *Status) IsDirty() bool { return s != nil && s.Worktree == WorktreeDirty }

// IsOutOfSync reports whether the checkout's default branch differs from
// origin: ahead, behind, or diverged.
func (s *Status) IsOutOfSync() bool { return s != nil && s.Sync.OutOfSync() }
