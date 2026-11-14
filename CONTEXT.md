# tig

A command-line companion for git and gh. Among other helpers, it keeps a declared set of repositories present and up to date on the user's machine.

## Language

### Keep

**Keep list**:
The `keep:` section of the tig config file: the user's declaration of which repositories should always exist on this machine.
_Avoid_: Manifest, repo list, keep config

**Kept repo**:
One entry in the keep list, declaring a remote repository and where its checkout should live. A kept repo exists whether or not its checkout does.
_Avoid_: Entry, managed repo, repo (when meaning the declaration)

**Checkout**:
The git working tree on disk at a kept repo's destination. May be missing.
_Avoid_: Clone (as a noun), local repo

**Destination**:
The filesystem path where a kept repo's checkout lives.
_Avoid_: Path, target, location

**Repo name**:
The `owner/repo` identifier of a kept repo, derived from its `repo` field (shorthand or full URL). Never includes the host. Unique within the keep list; used to select kept repos on the command line.
_Avoid_: ID, slug

**Default branch**:
The kept repo's declared `branch`, or the remote's HEAD branch when none is declared. `pull` only updates a checkout that is on this branch.
_Avoid_: Main branch, tracked branch

**Dirty**:
A checkout with staged or unstaged changes to tracked files. Untracked files do not make a checkout dirty.
_Avoid_: Modified, unclean

**Post-clone hook**:
A shell command a kept repo declares to run in its checkout right after tig freshly clones it.
_Avoid_: Setup script, install step

**Post-pull hook**:
A shell command a kept repo declares to run in its checkout after a pull that moved HEAD.
_Avoid_: Update hook

### Config

**Tig config**:
The single YAML file holding configuration for all tig features, each under its own top-level key.
_Avoid_: Settings, dotfile
