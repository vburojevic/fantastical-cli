# fantastical

A macOS‑only CLI wrapper around Fantastical’s URL handler and AppleScript integration.

## Why

- Fast event creation via natural language.
- Scriptable calendar view switching.
- Safe, machine‑readable outputs for automation.

## Install

### Homebrew (tap)

After the first tagged release:

```sh
brew tap vburojevic/tap
brew install fantastical-cli
```

Until then, install the latest commit:

```sh
brew install --HEAD vburojevic/tap/fantastical-cli
```

### Go install

```sh
go install github.com/vburojevic/fantastical-cli@latest
```

## Quickstart

```sh
fantastical parse --add --calendar "Work" --note "Alarm" "Wake up at 8am"
fantastical show --view month 2026-01-03
fantastical applescript --add "Daily standup at 9am"
fantastical eventkit calendars --json
```

## Flag ordering (important)

For `parse` (and `applescript`), put flags before the sentence, or use `--` to separate flags from the sentence. Otherwise flags are treated as part of the sentence.

```sh
fantastical parse --add --calendar "Personal" "Test event today at 22:00"
fantastical parse --add --calendar "Personal" -- "Test event today at 22:00"
```

## Commands

- `parse` — Build `x-fantastical3://parse?...` URLs
- `show` — Build `x-fantastical3://show/...` URLs
- `applescript` — Send a sentence to Fantastical via AppleScript
- `validate` — Validate parse/show input and print the URL
- `doctor` — Check Fantastical integration status
- `eventkit` — List calendars or events via EventKit (system Calendar access)
- `greta` — Machine‑readable CLI spec for agents
- `explain` — Human‑readable command walkthrough
- `man` — Manual page output (markdown or json)
- `completion` — Print/install/uninstall shell completions
- `help` — Show help for a command (use `--json` for machine output)

## Output modes

- `--json`: machine‑readable output (`command`, `url`, `open`, `copy`, `dry_run`).
- `--plain`: stable plain‑text output (just the URL).
- `--dry-run`: disable open/copy side effects.

## EventKit access

`eventkit` commands read calendars and events via EventKit. macOS will prompt for Calendar access on first use. The helper is compiled with `swiftc` (Xcode Command Line Tools) the first time you run an `eventkit` command. EventKit access requires macOS 14+ (uses the latest full‑access APIs).
`--refresh` is best-effort; remote calendars may still take time to sync.

Examples:

```sh
fantastical eventkit status --json
fantastical eventkit calendars --json
fantastical eventkit events --next-week --calendar "Work"
fantastical eventkit events --days 7 --query "standup"
fantastical eventkit events --calendar-id "ABC123" --format table --tz "America/Los_Angeles"
fantastical eventkit events --refresh --wait 20 --interval 2 --query "test"
```

## Input

- `--stdin` reads the sentence from stdin for `parse` and `applescript`.
- `--param key=value` passes additional Fantastical query params.
- `--timezone` sets `tz=...` on URL queries.

## Config

By default, config is loaded from:

- User config: `~/.config/fantastical/config.json` (or `$XDG_CONFIG_HOME`)
- Project config: `.fantastical.json` (in the current directory)

Set `FANTASTICAL_CONFIG` to override the user config path, or pass `--config` per command.

Example `config.json`:

```json
{
  "output": { "open": false, "print": true, "verbose": true },
  "parse": { "calendar": "Work", "add": true },
  "applescript": { "run": true }
}
```

Env overrides (highest precedence):

```
FANTASTICAL_DEFAULT_OPEN=1
FANTASTICAL_DEFAULT_PRINT=1
FANTASTICAL_DEFAULT_COPY=0
FANTASTICAL_DEFAULT_JSON=0
FANTASTICAL_DEFAULT_PLAIN=0
FANTASTICAL_DRY_RUN=0
FANTASTICAL_VERBOSE=1
FANTASTICAL_DEFAULT_CALENDAR=Work
FANTASTICAL_DEFAULT_NOTE=Alarm
FANTASTICAL_DEFAULT_ADD=1
FANTASTICAL_APPLESCRIPT_ADD=1
FANTASTICAL_APPLESCRIPT_RUN=1
FANTASTICAL_APPLESCRIPT_PRINT=0
FANTASTICAL_EVENTKIT_HELPER=/path/to/eventkit-helper
```

## AI agents (Codex, Claude Code)

Start here:

- `fantastical greta --format json` — full CLI spec
- `fantastical greta --examples` — curated examples
- `fantastical greta --capabilities` — supported views/features
- `fantastical help --json [command]` — command‑level JSON help
- `fantastical man --format json` — manual in JSON
- `fantastical validate --json <parse|show> ...` — safe validation

Agent docs: `docs/agent.md`

## Shell completion

```sh
# Print scripts
fantastical completion bash
fantastical completion zsh
fantastical completion fish

# Install to default user locations
fantastical completion install bash
fantastical completion install zsh
fantastical completion install fish

# Uninstall from default user locations
fantastical completion uninstall bash
fantastical completion uninstall zsh
fantastical completion uninstall fish

# Install to a specific path
fantastical completion install --path /usr/local/etc/bash_completion.d/fantastical bash
```

## Development

```sh
go test ./...
go build ./...
```

## Notes

- macOS only (Fantastical is a macOS app).
- `--open` defaults to true (uses `open <url>`).

## Docs

See `docs/index.md` for more details.

## License

MIT
