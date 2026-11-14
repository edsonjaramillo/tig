# tig keeps no state of its own

tig writes no marker, lock, or state files; every command derives what to do from the tig config file and the checkouts on disk. The cost is that some recovery is manual: a failed post-clone hook leaves a checkout that `clone` considers done, so the user re-runs it with `tig keep hooks <name>` (which is why `hooks` ships in the MVP). We accepted this to avoid state that can drift from reality.
