# Shell out to the git CLI only

tig drives repositories by executing the `git` binary, not through go-git and not through `gh repo clone`. Using the user's own git means credential helpers, SSH config, and auth behave exactly as they do at the terminal, and there is a single clone path to reason about. Non-interactive runs set `GIT_TERMINAL_PROMPT=0` and SSH `BatchMode=yes` so auth problems fail fast instead of hanging behind buffered parallel output.

## Considered Options

- **go-git**: no external dependency, but reimplements auth and diverges from the user's git behaviour.
- **`gh repo clone` for github.com**: convenient auth, but a second code path and a hard dependency on `gh` for a subset of repos.
