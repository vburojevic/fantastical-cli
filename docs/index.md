# fantastical CLI

A tiny CLI wrapper around Fantastical's URL handler and AppleScript integration.

macOS only (Fantastical is a macOS app).

## Commands

- `parse` — Build `x-fantastical3://parse?...` URLs
- `show` — Build `x-fantastical3://show/...` URLs
- `applescript` — Send a sentence to Fantastical via AppleScript
- `validate` — Validate parse/show input and print the URL
- `doctor` — Check Fantastical integration status
- `eventkit` — List calendars or events via EventKit (system Calendar access)
- `greta` — Machine-readable CLI spec for agents
- `explain` — Human-readable command walkthrough
- `man` — Manual page output (markdown or json)
- `completion` — Print/install/uninstall shell completions
- `help` — Show help for a command (use `--json` for machine output)

## Output modes

- `--json`: machine-readable output to stdout
- `--plain`: stable plain text output (URL only)
- `--print`: force URL output even when `--open` is on
- `--dry-run`: disable open/copy side effects

## Input

- `--stdin` reads the sentence from stdin for `parse` and `applescript`.
- `--param key=value` lets you pass additional Fantastical query params.
- `--timezone` sets `tz=...` on URL queries.
- For `parse`/`applescript`, place flags before the sentence or use `--` to separate.

## EventKit

`eventkit` commands access the system Calendar database via EventKit. macOS will prompt for Calendar access on first use. The helper is compiled with `swiftc` (Xcode Command Line Tools) the first time you run an `eventkit` command.

## Configuration

User config: `~/.config/fantastical/config.json` (or `$XDG_CONFIG_HOME`)
Project config: `.fantastical.json`

Precedence: flags > env > project config > user config.

Example:

```json
{
  "output": { "open": false, "print": true, "verbose": true },
  "parse": { "calendar": "Work", "add": true },
  "applescript": { "run": true }
}
```

## Examples

```sh
fantastical parse --print "Dinner with Sam tomorrow 7pm"
fantastical parse --stdin --json < input.txt
fantastical show --view month 2026-01-03
fantastical show --calendar-set "Work"
fantastical validate show month 2026-01-03
fantastical doctor --json
fantastical eventkit calendars --json
fantastical eventkit events --from 2026-01-03 --to 2026-01-04 --calendar "Work"
fantastical greta --format json
fantastical help --json parse
fantastical man --format json
```

## Agent docs

See `docs/agent.md` for agent-friendly discovery guidance.
