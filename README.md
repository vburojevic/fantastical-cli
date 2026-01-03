# fantastical

A tiny CLI wrapper around Fantastical's URL handler and AppleScript integration.

## Install

### Homebrew (tap)

This repo ships a Homebrew formula. Once this repo is on GitHub, you can:

```sh
brew tap vedranburojevic/fantastical-cli
brew install fantastical
```

To install the latest commit instead:

```sh
brew install --HEAD fantastical
```

### Go install

```sh
go install github.com/vedranburojevic/fantastical-cli@latest
```

## Usage

```sh
fantastical parse "Wake up at 8am" --add --calendar "Work" --note "Alarm"
fantastical parse --print "Dinner with Sam tomorrow 7pm"
fantastical show mini today
fantastical show calendar 2026-01-03
fantastical show set "My Calendar Set"
fantastical applescript --add "Wake up at 8am"
```

## Shell completion

```sh
# bash
fantastical completion bash > /usr/local/etc/bash_completion.d/fantastical

# zsh
fantastical completion zsh > /usr/local/share/zsh/site-functions/_fantastical

# fish
fantastical completion fish > ~/.config/fish/completions/fantastical.fish
```

## Notes

- On macOS, `--open` defaults to true (uses `open <url>`).
- On other OSes, `--open` defaults to false, so you'll typically use `--print`.

## License

MIT
