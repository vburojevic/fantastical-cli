# fantastical

A tiny CLI wrapper around Fantastical's URL handler and AppleScript integration.

## Install

### Homebrew (tap)

This repo ships a Homebrew formula. After the first tagged release, you can:

```sh
brew tap vburojevic/fantastical-cli
brew install fantastical
```

Until then, install the latest commit with:

```sh
brew install --HEAD fantastical
```

### Go install

```sh
go install github.com/vburojevic/fantastical-cli@latest
```

## Usage

```sh
fantastical parse "Wake up at 8am" --add --calendar "Work" --note "Alarm"
fantastical parse --print "Dinner with Sam tomorrow 7pm"
fantastical parse --stdin --json < input.txt
fantastical parse --param duration=60 --timezone "America/Los_Angeles" "Focus block"
fantastical show --view month 2026-01-03
fantastical show --calendar-set "My Calendar Set"
fantastical applescript --add "Wake up at 8am"
fantastical validate --json parse "Dinner at 7"
fantastical doctor
fantastical greta --format json
fantastical help --json parse
fantastical explain parse
fantastical man --format json
```

## Output modes

- `--json`: machine-readable output (`command`, `url`, `open`, `copy`, `dry_run`).
- `--plain`: stable plain text output (just the URL).

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
```

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

## Agent discovery

- `fantastical greta --format json` — full CLI spec
- `fantastical greta --examples` — curated examples
- `fantastical greta --capabilities` — supported views/features
- `fantastical help --json [command]` — command-level JSON help
- `fantastical man --format json` — manual in JSON

See `docs/agent.md` for more details.

## Notes

- macOS only (Fantastical is a macOS app).
- `--open` defaults to true (uses `open <url>`).
- `--dry-run` disables `--open` and `--copy` for safe previews.

## Docs

See `docs/index.md` for more details.

## License

MIT
