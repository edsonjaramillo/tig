# `tig keep` never deletes

`keep` only ever clones, fast-forwards, and runs hooks. Removing a kept repo from the keep list leaves its checkout untouched, and there is no `prune` command or `state: absent`. Deleting working trees risks losing unpushed work, and detecting orphans would require scanning the filesystem or recording state (see ADR-0002). Likewise, an existing destination that is not the expected repo is reported and skipped, never overwritten.
