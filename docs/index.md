# fantastical CLI

A tiny CLI wrapper around Fantastical's URL handler and AppleScript integration.

macOS only (Fantastical is a macOS app).

## Commands

- `parse` — Build `x-fantastical3://parse?...` URLs
- `show` — Build `x-fantastical3://show/...` URLs
- `applescript` — Send a sentence to Fantastical via AppleScript
- `validate` — Validate parse/show input and print the URL
- `doctor` — Check Fantastical integration status
- `greta` — Machine-readable CLI spec for agents
- `completion` — Print/install/uninstall shell completions

## Output modes

- `--json`: machine-readable output to stdout
- `--plain`: stable plain text output (URL only)
- `--print`: force URL output even when `--open` is on
- `--dry-run`: disable open/copy side effects

## Input

- `--stdin` reads the sentence from stdin for `parse` and `applescript`.
- `--param key=value` lets you pass additional Fantastical query params.
- `--timezone` sets `tz=...` on URL queries.

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
fantastical parse "Dinner with Sam tomorrow 7pm" --print
fantastical parse --stdin --json < input.txt
fantastical show --view month 2026-01-03
fantastical show --calendar-set "Work"
fantastical validate show month 2026-01-03
fantastical doctor --json
fantastical greta --format json
```
