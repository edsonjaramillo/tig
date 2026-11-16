# Use the gh CLI for GitHub-side operations

Operations on GitHub itself (looking up the active account, checking whether a repository exists, creating repositories) shell out to the `gh` binary in `internal/gh`. This reuses the user's existing gh authentication and active account instead of tig managing tokens or calling the REST API directly. Local repository work (cloning, remotes, pulls) still goes through the git CLI per ADR-0001; gh is never used to clone or to configure remotes, so remotes stay in tig's control (e.g. `tig repo create` always sets an HTTPS origin regardless of gh's `git_protocol`). gh runs with `GH_PROMPT_DISABLED=1`.

## Considered Options

- **GitHub REST API via net/http**: no gh dependency, but tig would need its own token discovery and storage.
- **`gh repo create --source --remote`**: one call, but the remote URL then follows gh's protocol setting and gh, not tig, owns the remote.
