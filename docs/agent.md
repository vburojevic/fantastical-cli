# Agent usage guide

This CLI is designed to be discoverable and safe for AI agents (Codex, Claude Code, etc.).

## Primary discovery entrypoints

- `fantastical greta --format json`
  - Full CLI spec with commands, flags, config, env, exit codes.
- `fantastical greta --examples`
  - Curated examples for common tasks.
- `fantastical greta --capabilities`
  - Supported views, output modes, and feature flags.
- `fantastical help --json [command]`
  - Command-level JSON help.
- `fantastical man --format json`
  - Full manual in JSON for tooling.
- `fantastical explain <command>`
  - Human-readable walkthroughs.

## Safe validation

Use `fantastical validate --json <parse|show> ...` to validate inputs without side effects.

## Structured output

Use `--json` with `parse`, `show`, `validate`, and `doctor` for machine-readable output.

## Configuration

- User config: `~/.config/fantastical/config.json` (or `$XDG_CONFIG_HOME`)
- Project config: `.fantastical.json`
- Env override: `FANTASTICAL_CONFIG`
- Precedence: flags > env > project config > user config.

## Notes

- macOS only (Fantastical is a macOS app).
- `--dry-run` disables opening/copying URLs.
