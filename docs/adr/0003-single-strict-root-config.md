# One strictly decoded config file for all of tig

All tig features share a single YAML file (`$XDG_CONFIG_HOME/tig.yaml`, falling back to `~/.config/tig.yaml` on every platform; overridable via `$TIG_CONFIG` / `--config`), each under its own top-level key. It is decoded into one root struct with unknown fields rejected, so a typo like `kep:` is an error instead of a silently ignored section. The consequence is that every new feature must register its key in the root struct. tig never re-serializes the file; `init` only appends raw text, so user comments survive.

## Considered Options

- **Decode only the `keep:` node strictly**: lets features stay independent but silently ignores misspelled top-level keys.
- **`os.UserConfigDir()`**: would put the file under `~/Library/Application Support` on macOS; rejected for a consistent dotfile location.
