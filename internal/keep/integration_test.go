package keep

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGit makes git ignore the user's global/system config and sets a
// commit identity.
func isolateGit(t *testing.T) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// remote is a bare repo at <tmp>/remotes/<owner>/<repo>.git plus a working
// clone used to push new commits to it.
type remote struct {
	url  string
	work string
}

func newRemote(t *testing.T, owner, repo string) remote {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "remotes", owner, repo+".git")
	work := filepath.Join(root, "work")
	gitT(t, root, "init", "-q", "--bare", "-b", "main", bare)
	gitT(t, root, "init", "-q", "-b", "main", work)
	writeFile(t, filepath.Join(work, "README"), "hello\n")
	gitT(t, work, "add", ".")
	gitT(t, work, "commit", "-q", "-m", "init")
	gitT(t, work, "remote", "add", "origin", bare)
	gitT(t, work, "push", "-q", "origin", "main")
	return remote{url: "file://" + bare, work: work}
}

func (r remote) commit(t *testing.T, file, content string) {
	t.Helper()
	writeFile(t, filepath.Join(r.work, file), content)
	gitT(t, r.work, "add", ".")
	gitT(t, r.work, "commit", "-q", "-m", "update "+file)
	gitT(t, r.work, "push", "-q", "origin", "main")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func kept(t *testing.T, r KeptRepo) KeptRepo {
	t.Helper()
	list := KeepList{Repos: []KeptRepo{r}}
	if err := list.Validate(); err != nil {
		t.Fatal(err)
	}
	return list.Repos[0]
}

func TestCloneLifecycle(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "code", "proj")
	r := kept(t, KeptRepo{
		Repo:        rem.url,
		Destination: dest,
		PostClone:   []string{`printf '%s %s' "$TIG_REPO_NAME" "$TIG_DEFAULT_BRANCH" > hook.out`},
	})
	if r.Name != "me/proj" {
		t.Fatalf("name = %q", r.Name)
	}

	res := CloneOp(Options{DryRun: true})(ctx, r)
	if res.Outcome != "would clone" || !strings.Contains(res.Output, "git clone") || !strings.Contains(res.Output, "hook: printf") {
		t.Fatalf("dry run = %+v", res)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("dry run created the destination")
	}

	res = CloneOp(Options{})(ctx, r)
	if res.Outcome != Cloned || res.Err != nil {
		t.Fatalf("clone = %+v", res)
	}
	out, err := os.ReadFile(filepath.Join(dest, "hook.out"))
	if err != nil || string(out) != "me/proj main" {
		t.Fatalf("hook output = %q, %v", out, err)
	}

	res = CloneOp(Options{})(ctx, r)
	if res.Outcome != Present {
		t.Fatalf("second clone = %+v", res)
	}
}

func TestCloneNoHooksAndBranch(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	gitT(t, rem.work, "switch", "-q", "-c", "dev")
	rem.commit(t, "dev.txt", "dev\n")
	gitT(t, rem.work, "push", "-q", "origin", "dev")

	dest := filepath.Join(t.TempDir(), "proj")
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest, Branch: "dev", PostClone: []string{"touch hook.out"}})
	res := CloneOp(Options{NoHooks: true})(ctx, r)
	if res.Outcome != Cloned {
		t.Fatalf("clone = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "hook.out")); !os.IsNotExist(err) {
		t.Error("hook ran despite NoHooks")
	}
	if b := currentBranch(ctx, dest); b != "dev" {
		t.Errorf("branch = %q, want dev", b)
	}
}

func TestCloneHookFailureThenHooks(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "proj")
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest, PostClone: []string{"echo checking", "test -f ok", "touch done"}})

	res := CloneOp(Options{})(ctx, r)
	if !res.Failed() || res.Detail != "cloned, hook failed" || res.Hint != "run: tig keep hooks me/proj" {
		t.Fatalf("clone = %+v", res)
	}
	if !strings.Contains(res.Output, "checking") {
		t.Errorf("hook output not captured: %q", res.Output)
	}
	if _, err := os.Stat(filepath.Join(dest, "README")); err != nil {
		t.Fatal("checkout was not left in place")
	}
	if _, err := os.Stat(filepath.Join(dest, "done")); !os.IsNotExist(err) {
		t.Fatal("hooks continued after failure")
	}

	writeFile(t, filepath.Join(dest, "ok"), "")
	res = HooksOp(Options{}, PostClone)(ctx, r)
	if res.Outcome != HooksRan || res.Err != nil {
		t.Fatalf("hooks = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "done")); err != nil {
		t.Fatal("hooks did not complete")
	}
}

func TestCloneRefusesForeignDestination(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	other := newRemote(t, "me", "other")

	// Not a git checkout.
	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(plain, "keep.txt"), "mine")
	res := CloneOp(Options{})(ctx, kept(t, KeptRepo{Repo: rem.url, Destination: plain}))
	if !res.Failed() || !strings.Contains(res.Err.Error(), "not a git checkout") {
		t.Fatalf("plain dir = %+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(plain, "keep.txt")); string(b) != "mine" {
		t.Fatal("existing content was touched")
	}

	// A checkout of a different repo.
	wrong := filepath.Join(t.TempDir(), "wrong")
	gitT(t, "", "clone", "-q", other.url, wrong)
	res = CloneOp(Options{})(ctx, kept(t, KeptRepo{Repo: rem.url, Destination: wrong}))
	if !res.Failed() || !strings.Contains(res.Err.Error(), "expected") {
		t.Fatalf("wrong origin = %+v", res)
	}

	// Status reports the same failure.
	res = StatusOp(false)(ctx, kept(t, KeptRepo{Repo: rem.url, Destination: wrong}))
	if !res.Failed() {
		t.Fatalf("status of wrong origin = %+v", res)
	}
}

func TestStatusAndPull(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "proj")
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest, PostPull: []string{"touch pulled.out"}})

	if res := StatusOp(false)(ctx, r); res.Status.Worktree != "missing" || res.Status.Sync.String() != "" || res.Failed() {
		t.Fatalf("missing status = %+v %+v", res, res.Status)
	}
	if res := PullOp(Options{})(ctx, r); res.Outcome != Skipped || res.Detail != "skipped: missing" || res.Failed() {
		t.Fatalf("pull missing = %+v", res)
	}

	gitT(t, "", "clone", "-q", rem.url, dest)
	if res := StatusOp(false)(ctx, r); res.Status.Worktree != "clean" || res.Status.Sync.String() != "up-to-date" || res.Status.Branch != "main" {
		t.Fatalf("status = %+v", res.Status)
	}
	if res := PullOp(Options{})(ctx, r); res.Outcome != UpToDate {
		t.Fatalf("pull up to date = %+v", res)
	}

	rem.commit(t, "new.txt", "new\n")
	if res := StatusOp(false)(ctx, r); res.Status.Sync.String() != "up-to-date" {
		t.Fatalf("status without fetch should not see remote change: %+v", res.Status)
	}
	if res := StatusOp(true)(ctx, r); res.Status.Sync.String() != "behind 1" {
		t.Fatalf("status --fetch = %+v", res.Status)
	}

	if res := PullOp(Options{DryRun: true})(ctx, r); res.Outcome != "would pull" || !strings.Contains(res.Output, "merge --ff-only origin/main") {
		t.Fatalf("pull dry run = %+v", res)
	}

	// Dirty: tracked change.
	writeFile(t, filepath.Join(dest, "README"), "changed\n")
	if res := PullOp(Options{})(ctx, r); res.Detail != "skipped: dirty" {
		t.Fatalf("pull dirty = %+v", res)
	}
	if res := StatusOp(false)(ctx, r); res.Status.Worktree != "dirty" {
		t.Fatalf("status dirty = %+v", res.Status)
	}
	gitT(t, dest, "checkout", "--", "README")

	// Untracked files do not make a checkout dirty.
	writeFile(t, filepath.Join(dest, "untracked"), "x")

	// Off branch.
	gitT(t, dest, "switch", "-q", "-c", "feature-x")
	if res := PullOp(Options{})(ctx, r); res.Detail != "skipped: on feature-x, not main" {
		t.Fatalf("pull off-branch = %+v", res)
	}
	if res := StatusOp(false)(ctx, r); res.Status.Sync.String() != "off-branch (feature-x)" {
		t.Fatalf("status off-branch = %+v", res.Status)
	}
	gitT(t, dest, "switch", "-q", "main")

	res := PullOp(Options{})(ctx, r)
	if res.Outcome != Pulled || res.Failed() {
		t.Fatalf("pull = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "new.txt")); err != nil {
		t.Fatal("pull did not fast-forward")
	}
	if _, err := os.Stat(filepath.Join(dest, "pulled.out")); err != nil {
		t.Fatal("post-pull hook did not run")
	}
	_ = os.Remove(filepath.Join(dest, "pulled.out"))
	if res := PullOp(Options{})(ctx, r); res.Outcome != UpToDate {
		t.Fatalf("second pull = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "pulled.out")); !os.IsNotExist(err) {
		t.Fatal("post-pull hook ran although HEAD did not move")
	}

	// Ahead, then diverged.
	writeFile(t, filepath.Join(dest, "local.txt"), "local\n")
	gitT(t, dest, "add", "local.txt")
	gitT(t, dest, "commit", "-q", "-m", "local")
	if res := StatusOp(false)(ctx, r); res.Status.Sync.String() != "ahead 1" {
		t.Fatalf("status ahead = %+v", res.Status)
	}
	rem.commit(t, "more.txt", "more\n")
	if res := StatusOp(true)(ctx, r); res.Status.Sync.String() != "diverged" {
		t.Fatalf("status diverged = %+v", res.Status)
	}
	if res := PullOp(Options{})(ctx, r); !res.Failed() || !strings.Contains(res.Err.Error(), "diverged") {
		t.Fatalf("pull diverged = %+v", res)
	}
}

func TestDefaultBranchSetHead(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "proj")
	gitT(t, "", "clone", "-q", rem.url, dest)
	gitT(t, dest, "remote", "set-head", "origin", "--delete")
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest})

	if b, err := DefaultBranch(ctx, r, false); b != "" || err != nil {
		t.Fatalf("offline default branch = %q, %v", b, err)
	}
	if res := StatusOp(false)(ctx, r); res.Status.Sync.String() != "no-upstream" {
		t.Fatalf("status = %+v", res.Status)
	}
	if b, err := DefaultBranch(ctx, r, true); b != "main" || err != nil {
		t.Fatalf("online default branch = %q, %v", b, err)
	}
}

func TestRunOp(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "proj")
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest})

	if res := RunOp(Options{}, []string{"pwd"}, false)(ctx, r); res.Outcome != Skipped {
		t.Fatalf("run missing = %+v", res)
	}
	gitT(t, "", "clone", "-q", rem.url, dest)
	res := RunOp(Options{}, []string{"sh", "-c", `echo "$TIG_REPO_NAME"; cat README`}, false)(ctx, r)
	if res.Outcome != Ran || res.Output != "me/proj\nhello\n" {
		t.Fatalf("run = %+v", res)
	}
	if res := RunOp(Options{}, []string{"false"}, false)(ctx, r); !res.Failed() {
		t.Fatalf("run false = %+v", res)
	}
	if res := RunOp(Options{DryRun: true}, []string{"echo", "hi"}, false)(ctx, r); res.Outcome != "would run" || res.Output != "echo hi" {
		t.Fatalf("run dry = %+v", res)
	}
}

func TestRunOpShell(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "proj")
	gitT(t, "", "clone", "-q", rem.url, dest)
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest})

	res := RunOp(Options{}, []string{"echo", "$TIG_REPO_NAME", "|", "tr", "a-z", "A-Z"}, true)(ctx, r)
	if res.Outcome != Ran || res.Output != "ME/PROJ\n" {
		t.Fatalf("run --shell = %+v", res)
	}
	res = RunOp(Options{DryRun: true}, []string{"cat README | wc -l"}, true)(ctx, r)
	if res.Outcome != WouldRun || res.Output != `sh -c "cat README | wc -l"` {
		t.Fatalf("run --shell dry = %+v", res)
	}
}

func TestHooksOpPostPull(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	rem := newRemote(t, "me", "proj")
	dest := filepath.Join(t.TempDir(), "proj")
	gitT(t, "", "clone", "-q", rem.url, dest)
	r := kept(t, KeptRepo{Repo: rem.url, Destination: dest, PostClone: []string{"touch clone.out"}, PostPull: []string{"touch pull.out"}})

	if res := HooksOp(Options{}, PostPull)(ctx, r); res.Outcome != HooksRan || res.Failed() {
		t.Fatalf("hooks --post-pull = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "pull.out")); err != nil {
		t.Fatal("post_pull hook did not run")
	}
	if _, err := os.Stat(filepath.Join(dest, "clone.out")); !os.IsNotExist(err) {
		t.Fatal("post_clone hook ran for --post-pull")
	}

	none := kept(t, KeptRepo{Repo: rem.url, Destination: dest})
	if res := HooksOp(Options{}, PostPull)(ctx, none); res.Outcome != NoHooks || res.Label() != "no post_pull hooks" {
		t.Fatalf("no hooks = %+v", res)
	}
}
