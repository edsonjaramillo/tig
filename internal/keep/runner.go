package keep

import (
	"context"
	"strconv"
	"strings"
	"sync"
)

// Outcome is the result category of one operation on one kept repo. Outcomes
// are counted in the summary line.
type Outcome string

// Outcomes.
const (
	Cloned   Outcome = "cloned"
	Present  Outcome = "present"
	HooksRan Outcome = "hooks ran"
	NoHooks  Outcome = "no hooks"
	Pulled   Outcome = "pulled"
	UpToDate Outcome = "up-to-date"
	Ran      Outcome = "ran"
	Skipped  Outcome = "skipped"
	Failed   Outcome = "failed"

	// Status outcomes count checkouts by worktree state.
	Missing Outcome = Outcome(WorktreeMissing)
	Clean   Outcome = Outcome(WorktreeClean)
	Dirty   Outcome = Outcome(WorktreeDirty)

	// Dry-run outcomes.
	WouldClone    Outcome = "would clone"
	WouldRunHooks Outcome = "would run hooks"
	WouldPull     Outcome = "would pull"
	WouldRun      Outcome = "would run"
)

// Result is the outcome of one operation on one kept repo.
type Result struct {
	Repo    KeptRepo
	Outcome Outcome // summary category
	// Detail replaces the outcome in the per-repo line when set,
	// e.g. "skipped: on feature-x, not main".
	Detail string
	Err    error
	// Output is buffered command output (hook output, run output) or, for
	// dry runs, the planned steps.
	Output string
	// Hint is a follow-up suggestion, e.g. "run: tig keep hooks owner/repo".
	Hint string
	// Status is set by the status action.
	Status *Status
}

// Failed reports whether the result is a real failure.
func (r Result) Failed() bool { return r.Err != nil || r.Outcome == Failed }

// Label is the text shown for the result: Detail, or the outcome.
func (r Result) Label() string {
	if r.Detail != "" {
		return r.Detail
	}
	return string(r.Outcome)
}

// Op performs one operation on one kept repo.
type Op func(ctx context.Context, r KeptRepo) Result

// Run applies op to every repo using up to jobs workers. emit, if non-nil, is
// called with each result in keep list order as soon as it and all earlier
// results are done, so output streams deterministically. Results are returned
// in keep list order.
func Run(ctx context.Context, repos []KeptRepo, jobs int, op Op, emit func(Result)) []Result {
	if jobs < 1 {
		jobs = 1
	}
	results := make([]Result, len(repos))
	done := make([]bool, len(repos))
	var mu sync.Mutex
	next := 0
	flush := func() {
		for next < len(repos) && done[next] {
			if emit != nil {
				emit(results[next])
			}
			next++
		}
	}

	work := make(chan int)
	var wg sync.WaitGroup
	for range min(jobs, len(repos)) {
		wg.Go(func() {
			for i := range work {
				res := op(ctx, repos[i])
				res.Repo = repos[i]
				mu.Lock()
				results[i], done[i] = res, true
				flush()
				mu.Unlock()
			}
		})
	}
	for i := range repos {
		work <- i
	}
	close(work)
	wg.Wait()
	return results
}

// Summary returns e.g. "cloned 2 · present 5 · skipped 1 · failed 1",
// counting outcomes in order of first appearance. extra counts (such as
// ignored names) are appended when non-zero.
func Summary(results []Result, extra ...Count) string {
	var order []Outcome
	counts := map[Outcome]int{}
	for _, r := range results {
		o := r.Outcome
		if r.Err != nil {
			o = Failed
		}
		if counts[o] == 0 {
			order = append(order, o)
		}
		counts[o]++
	}
	parts := make([]string, 0, len(order)+len(extra))
	for _, o := range order {
		parts = append(parts, string(o)+" "+strconv.Itoa(counts[o]))
	}
	for _, c := range extra {
		if c.N > 0 {
			parts = append(parts, c.Label+" "+strconv.Itoa(c.N))
		}
	}
	if len(parts) == 0 {
		return "nothing to do"
	}
	return strings.Join(parts, " · ")
}

// Count is an extra labeled count for Summary.
type Count struct {
	Label string
	N     int
}
