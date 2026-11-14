package keep

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func TestRunEmitsInOrder(t *testing.T) {
	repos := []KeptRepo{{Name: "a/1"}, {Name: "a/2"}, {Name: "a/3"}, {Name: "a/4"}}
	op := func(_ context.Context, r KeptRepo) Result {
		// Finish in reverse order.
		time.Sleep(time.Duration(5-int(r.Name[2]-'0')) * 5 * time.Millisecond)
		return Result{Outcome: Present}
	}
	var emitted []string
	results := Run(context.Background(), repos, 4, op, func(r Result) { emitted = append(emitted, r.Repo.Name) })
	want := []string{"a/1", "a/2", "a/3", "a/4"}
	if !slices.Equal(emitted, want) {
		t.Errorf("emitted %v, want %v", emitted, want)
	}
	for i, r := range results {
		if r.Repo.Name != want[i] {
			t.Errorf("results[%d] = %s", i, r.Repo.Name)
		}
	}
}

func TestSummary(t *testing.T) {
	results := []Result{
		{Outcome: Cloned},
		{Outcome: Present},
		{Outcome: Cloned},
		{Outcome: Skipped},
		{Outcome: Cloned, Err: errors.New("x")},
	}
	got := Summary(results, Count{"ignored", 1}, Count{"zero", 0})
	want := "cloned 2 · present 1 · skipped 1 · failed 1 · ignored 1"
	if got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
	if Summary(nil) != "nothing to do" {
		t.Errorf("empty summary = %q", Summary(nil))
	}
}
