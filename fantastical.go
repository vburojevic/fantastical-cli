//go:build darwin
// +build darwin

// fantastical.go
//
// A tiny CLI wrapper around Fantastical's documented integrations:
// - URL handler: x-fantastical3://parse and x-fantastical3://show/...
// - AppleScript: tell application "Fantastical" parse sentence "..."
//
// Build:
//
//	go build -o fantastical fantastical.go
//
// Examples:
//
//	./fantastical parse "Wake up at 8am" --add --calendar "Work" --note "Alarm"
//	./fantastical show mini today
//	./fantastical show calendar 2026-01-03
//	./fantastical show set "My Calendar Set"
//	./fantastical applescript --add "Wake up at 8am"
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	appName           = "fantastical"
	fantasticalScheme = "x-fantastical3://"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

var errUsage = errors.New("usage")

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) < 2 {
		usage(errOut)
		return 2
	}

	cmd := strings.ToLower(args[1])
	var err error

	switch cmd {
	case "parse":
		err = cmdParse(args[2:], in, out, errOut)
	case "show":
		err = cmdShow(args[2:], out, errOut)
	case "applescript", "as":
		err = cmdAppleScript(args[2:], in, out, errOut)
	case "completion":
		err = cmdCompletion(args[2:], out, errOut)
	case "help", "-h", "--help":
		if len(args) > 2 {
			err = printSubcommandHelp(args[2], out)
		} else {
			usage(out)
		}
	case "version", "--version":
		printVersion(out)
	default:
		fmt.Fprintf(errOut, "Unknown command: %q\n\n", cmd)
		usage(errOut)
		return 2
	}

	if err != nil {
		fmt.Fprintln(errOut, "Error:", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}

	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `fantastical - CLI for Fantastical URL handler + AppleScript integration

USAGE
  fantastical [--version] <command> [flags] [args]

COMMANDS
  parse        Build (and optionally open) x-fantastical3://parse?... URLs
  show         Build (and optionally open) x-fantastical3://show/... URLs
  applescript  Send "parse sentence" to Fantastical via osascript (macOS)
  completion   Print or install shell completion (bash|zsh|fish)
  help         Show help for a command
  version      Print version information

NOTES
  - macOS only (Fantastical is a macOS app).
  - --open defaults to true (uses "open <url>").
  - Use --json for machine-readable output; use --plain for stable text output.

EXAMPLES
  fantastical parse "Wake up at 8am" --add --calendar "Work" --note "Alarm"
  fantastical parse --print "Dinner with Sam tomorrow 7pm"
  fantastical parse --stdin --json < input.txt
  fantastical show mini today
  fantastical show month 2026-01-03
  fantastical show set "My Calendar Set"
  fantastical applescript --add "Wake up at 8am"
`)
}

func printSubcommandHelp(cmd string, w io.Writer) error {
	switch strings.ToLower(cmd) {
	case "parse":
		fs, _ := newParseFlagSet(w, defaultParseOptions(nil))
		fs.Usage()
		return nil
	case "show":
		fs, _ := newShowFlagSet(w, defaultShowOptions(nil))
		fs.Usage()
		return nil
	case "applescript", "as":
		fs, _ := newAppleScriptFlagSet(w, defaultAppleScriptOptions(nil))
		fs.Usage()
		return nil
	case "completion":
		fs, _ := newCompletionFlagSet(w)
		fs.Usage()
		return nil
	default:
		return fmt.Errorf("%w: unknown help topic %q", errUsage, cmd)
	}
}

func versionString() string {
	v := version
	if strings.TrimSpace(v) == "" {
		v = "dev"
	}

	parts := []string{v}
	if strings.TrimSpace(commit) != "" {
		parts = append(parts, commit)
	}
	if strings.TrimSpace(date) != "" {
		parts = append(parts, date)
	}
	return strings.Join(parts, " ")
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s %s\n", appName, versionString())
}

type outputOptions struct {
	open    bool
	print   bool
	copy    bool
	json    bool
	plain   bool
	dryRun  bool
	verbose bool
}

type parseOptions struct {
	outputOptions
	note     string
	calendar string
	add      bool
	stdin    bool
	params   stringSlice
}

func defaultOutputOptions(cfg *Config) outputOptions {
	opts := outputOptions{
		open: true,
	}
	if cfg == nil {
		return opts
	}
	if cfg.Output.Open != nil {
		opts.open = *cfg.Output.Open
	}
	if cfg.Output.Print != nil {
		opts.print = *cfg.Output.Print
	}
	if cfg.Output.Copy != nil {
		opts.copy = *cfg.Output.Copy
	}
	if cfg.Output.JSON != nil {
		opts.json = *cfg.Output.JSON
	}
	if cfg.Output.Plain != nil {
		opts.plain = *cfg.Output.Plain
	}
	if cfg.Output.DryRun != nil {
		opts.dryRun = *cfg.Output.DryRun
	}
	if cfg.Output.Verbose != nil {
		opts.verbose = *cfg.Output.Verbose
	}
	return opts
}

func defaultParseOptions(cfg *Config) parseOptions {
	opts := parseOptions{
		outputOptions: defaultOutputOptions(cfg),
	}
	if cfg == nil {
		return opts
	}
	if strings.TrimSpace(cfg.Parse.Calendar) != "" {
		opts.calendar = cfg.Parse.Calendar
	}
	if strings.TrimSpace(cfg.Parse.Note) != "" {
		opts.note = cfg.Parse.Note
	}
	if cfg.Parse.Add != nil {
		opts.add = *cfg.Parse.Add
	}
	return opts
}

func newParseFlagSet(w io.Writer, defaults parseOptions) (*flag.FlagSet, *parseOptions) {
	opts := defaults

	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we'll print our own usage on error/help

	fs.StringVar(&opts.note, "note", opts.note, "Optional note (maps to n=...)")
	fs.StringVar(&opts.note, "n", opts.note, "Alias for --note")

	fs.StringVar(&opts.calendar, "calendar", opts.calendar, "Optional calendar name (maps to calendarName=...)")
	fs.StringVar(&opts.calendar, "calendarName", opts.calendar, "Alias for --calendar")

	fs.BoolVar(&opts.add, "add", opts.add, "Add immediately without interaction (maps to add=1)")
	fs.BoolVar(&opts.open, "open", opts.open, "Open the generated URL via system opener")
	fs.BoolVar(&opts.print, "print", opts.print, "Print the generated URL to stdout")
	fs.BoolVar(&opts.copy, "copy", opts.copy, "Copy the generated URL to clipboard (pbcopy/wl-copy/xclip/clip, if available)")
	fs.BoolVar(&opts.json, "json", opts.json, "Print machine-readable JSON output")
	fs.BoolVar(&opts.plain, "plain", opts.plain, "Print stable plain-text output")
	fs.BoolVar(&opts.dryRun, "dry-run", opts.dryRun, "Preview only; do not open or copy")
	fs.BoolVar(&opts.verbose, "verbose", opts.verbose, "Verbose output to stderr")
	fs.BoolVar(&opts.stdin, "stdin", false, "Read sentence from stdin instead of args")
	fs.Var(&opts.params, "param", "Extra Fantastical query param (key=value), repeatable")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical parse [flags] <sentence...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical parse \"Wake up at 8am\" --add --calendar Work --note \"Alarm\"")
	}

	return fs, &opts
}

func cmdParse(args []string, in io.Reader, out, errOut io.Writer) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fs, opts := newParseFlagSet(errOut, defaultParseOptions(cfg))

	if err := fs.Parse(args); err != nil {
		// flag package returns flag.ErrHelp if -h/-help is used.
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	if opts.json && opts.plain {
		return fmt.Errorf("%w: --json and --plain are mutually exclusive", errUsage)
	}

	sentence, err := readSentence(fs.Args(), opts.stdin, in)
	if err != nil {
		fs.Usage()
		return err
	}

	extraParams, err := parseParams(opts.params)
	if err != nil {
		fs.Usage()
		return err
	}

	u := buildParseURL(sentence, opts.note, opts.calendar, opts.add, extraParams)

	if opts.dryRun {
		opts.open = false
		opts.copy = false
	}

	logVerbose(errOut, opts.verbose, "url: %s", u)
	logVerbose(errOut, opts.verbose, "open=%t copy=%t dry-run=%t", opts.open, opts.copy, opts.dryRun)

	didOutput := false

	if opts.json {
		payload := map[string]any{
			"command":  "parse",
			"sentence": sentence,
			"url":      u,
			"open":     opts.open,
			"copy":     opts.copy,
			"dry_run":  opts.dryRun,
		}
		if err := writeJSON(out, payload); err != nil {
			return err
		}
		didOutput = true
	} else if opts.print || opts.plain || (!opts.open && !opts.copy) {
		fmt.Fprintln(out, u)
		didOutput = true
	}

	if opts.copy {
		if err := copyToClipboard(u); err != nil {
			return err
		}
	}
	if opts.open {
		if err := openURL(u, out, errOut); err != nil {
			return err
		}
	}

	if !didOutput && !opts.open && !opts.copy {
		fmt.Fprintln(out, u)
	}

	return nil
}

type showOptions struct {
	outputOptions
	params stringSlice
}

func defaultShowOptions(cfg *Config) showOptions {
	return showOptions{outputOptions: defaultOutputOptions(cfg)}
}

func newShowFlagSet(w io.Writer, defaults showOptions) (*flag.FlagSet, *showOptions) {
	opts := defaults

	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.BoolVar(&opts.open, "open", opts.open, "Open the generated URL via system opener")
	fs.BoolVar(&opts.print, "print", opts.print, "Print the generated URL to stdout")
	fs.BoolVar(&opts.copy, "copy", opts.copy, "Copy the generated URL to clipboard (pbcopy/wl-copy/xclip/clip, if available)")
	fs.BoolVar(&opts.json, "json", opts.json, "Print machine-readable JSON output")
	fs.BoolVar(&opts.plain, "plain", opts.plain, "Print stable plain-text output")
	fs.BoolVar(&opts.dryRun, "dry-run", opts.dryRun, "Preview only; do not open or copy")
	fs.BoolVar(&opts.verbose, "verbose", opts.verbose, "Verbose output to stderr")
	fs.Var(&opts.params, "param", "Extra Fantastical query param (key=value), repeatable")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical show [flags] <view> [yyyy-mm-dd|today|tomorrow|yesterday]\n  fantastical show [flags] set <calendar-set-name...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical show mini today\n  fantastical show calendar 2026-01-03\n  fantastical show month 2026-01-03\n  fantastical show set \"My Calendar Set\"")
	}

	return fs, &opts
}

func cmdShow(args []string, out, errOut io.Writer) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fs, opts := newShowFlagSet(errOut, defaultShowOptions(cfg))

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	if opts.json && opts.plain {
		return fmt.Errorf("%w: --json and --plain are mutually exclusive", errUsage)
	}

	rest := fs.Args()
	if len(rest) < 1 {
		fs.Usage()
		return fmt.Errorf("%w: missing view", errUsage)
	}

	sub := strings.ToLower(rest[0])

	extraParams, err := parseParams(opts.params)
	if err != nil {
		fs.Usage()
		return err
	}

	var u string
	switch sub {
	case "set":
		if len(rest) < 2 {
			fs.Usage()
			return fmt.Errorf("%w: missing calendar set name", errUsage)
		}
		name := strings.TrimSpace(strings.Join(rest[1:], " "))
		q := url.Values{}
		q.Set("name", name)
		for key, vals := range extraParams {
			for _, v := range vals {
				q.Add(key, v)
			}
		}
		u = fantasticalScheme + "show/set?" + encodeQuery(q)
	default:
		if len(rest) > 2 {
			fs.Usage()
			return fmt.Errorf("%w: too many args for %q; expected: fantastical show %s [date]", errUsage, sub, sub)
		}
		u = fantasticalScheme + "show/" + sub
		if len(rest) == 2 {
			d, err := parseDateArg(rest[1])
			if err != nil {
				return err
			}
			u = u + "/" + d.Format("2006-01-02")
		}
		if len(extraParams) > 0 {
			u = u + "?" + encodeQuery(extraParams)
		}
	}

	if opts.dryRun {
		opts.open = false
		opts.copy = false
	}

	logVerbose(errOut, opts.verbose, "url: %s", u)
	logVerbose(errOut, opts.verbose, "open=%t copy=%t dry-run=%t", opts.open, opts.copy, opts.dryRun)

	didOutput := false

	if opts.json {
		payload := map[string]any{
			"command": "show",
			"view":    sub,
			"url":     u,
			"open":    opts.open,
			"copy":    opts.copy,
			"dry_run": opts.dryRun,
		}
		if err := writeJSON(out, payload); err != nil {
			return err
		}
		didOutput = true
	} else if opts.print || opts.plain || (!opts.open && !opts.copy) {
		fmt.Fprintln(out, u)
		didOutput = true
	}

	if opts.copy {
		if err := copyToClipboard(u); err != nil {
			return err
		}
	}
	if opts.open {
		if err := openURL(u, out, errOut); err != nil {
			return err
		}
	}

	if !didOutput && !opts.open && !opts.copy {
		fmt.Fprintln(out, u)
	}

	return nil
}

type appleScriptOptions struct {
	add     bool
	run     bool
	print   bool
	dryRun  bool
	verbose bool
	stdin   bool
}

func defaultAppleScriptOptions(cfg *Config) appleScriptOptions {
	opts := appleScriptOptions{
		run: true,
	}
	if cfg == nil {
		return opts
	}
	if cfg.AppleScript.Add != nil {
		opts.add = *cfg.AppleScript.Add
	}
	if cfg.AppleScript.Run != nil {
		opts.run = *cfg.AppleScript.Run
	}
	if cfg.AppleScript.Print != nil {
		opts.print = *cfg.AppleScript.Print
	}
	if cfg.Output.DryRun != nil {
		opts.dryRun = *cfg.Output.DryRun
	}
	if cfg.Output.Verbose != nil {
		opts.verbose = *cfg.Output.Verbose
	}
	return opts
}

func newAppleScriptFlagSet(w io.Writer, defaults appleScriptOptions) (*flag.FlagSet, *appleScriptOptions) {
	opts := defaults

	fs := flag.NewFlagSet("applescript", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.BoolVar(&opts.add, "add", opts.add, "Use Fantastical AppleScript 'with add immediately'")
	fs.BoolVar(&opts.run, "run", opts.run, "Run osascript (macOS only)")
	fs.BoolVar(&opts.print, "print", opts.print, "Print the AppleScript instead of (or in addition to) running it")
	fs.BoolVar(&opts.dryRun, "dry-run", opts.dryRun, "Preview only; do not run osascript")
	fs.BoolVar(&opts.verbose, "verbose", opts.verbose, "Verbose output to stderr")
	fs.BoolVar(&opts.stdin, "stdin", false, "Read sentence from stdin instead of args")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical applescript|as [flags] <sentence...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical applescript --add \"Wake up at 8am\"")
	}

	return fs, &opts
}

func cmdAppleScript(args []string, in io.Reader, out, errOut io.Writer) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fs, opts := newAppleScriptFlagSet(errOut, defaultAppleScriptOptions(cfg))

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	if opts.dryRun {
		opts.run = false
		if !opts.print {
			opts.print = true
		}
	}

	sentence, err := readSentence(fs.Args(), opts.stdin, in)
	if err != nil {
		fs.Usage()
		return err
	}

	scriptLines := []string{
		"on run argv",
		"set theSentence to item 1 of argv",
		"set addImmediately to false",
		"if (count of argv) > 1 then",
		"set addImmediately to (item 2 of argv is \"1\")",
		"end if",
		"tell application \"Fantastical\"",
		"if addImmediately then",
		"parse sentence theSentence with add immediately",
		"else",
		"parse sentence theSentence",
		"end if",
		"end tell",
		"end run",
	}

	if opts.print || !opts.run {
		fmt.Fprintln(out, strings.Join(scriptLines, "\n"))
	}

	if !opts.run {
		return nil
	}

	logVerbose(errOut, opts.verbose, "running osascript (add=%t)", opts.add)

	osascriptArgs := make([]string, 0, len(scriptLines)*2+3)
	for _, line := range scriptLines {
		osascriptArgs = append(osascriptArgs, "-e", line)
	}
	addArg := "0"
	if opts.add {
		addArg = "1"
	}
	osascriptArgs = append(osascriptArgs, "--", sentence, addArg)

	cmdName := osascriptCommand()
	cmd := exec.Command(cmdName, osascriptArgs...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

type completionOptions struct{}

type completionInstallOptions struct {
	path string
}

func newCompletionFlagSet(w io.Writer) (*flag.FlagSet, *completionOptions) {
	opts := &completionOptions{}

	fs := flag.NewFlagSet("completion", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical completion [bash|zsh|fish]\n  fantastical completion install [--path <path>] [bash|zsh|fish]\n")
		fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical completion zsh\n  fantastical completion install fish")
	}

	return fs, opts
}

func cmdCompletion(args []string, out, errOut io.Writer) error {
	fs, _ := newCompletionFlagSet(errOut)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	rest := fs.Args()
	if len(rest) < 1 {
		fs.Usage()
		return fmt.Errorf("%w: missing shell name (bash|zsh|fish)", errUsage)
	}

	if rest[0] == "install" {
		return cmdCompletionInstall(rest[1:], out, errOut)
	}

	shell := strings.ToLower(strings.TrimSpace(rest[0]))
	script, err := completionScript(shell)
	if err != nil {
		fs.Usage()
		return err
	}

	fmt.Fprintln(out, script)
	return nil
}

func cmdCompletionInstall(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("completion install", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := completionInstallOptions{}
	fs.StringVar(&opts.path, "path", "", "Install path (defaults to a user-local location)")

	fs.Usage = func() {
		fmt.Fprint(errOut, "USAGE:\n  fantastical completion install [--path <path>] [bash|zsh|fish]\n")
		fmt.Fprintln(errOut, "\nEXAMPLE:\n  fantastical completion install zsh")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("%w: missing shell name (bash|zsh|fish)", errUsage)
	}

	shell := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	script, err := completionScript(shell)
	if err != nil {
		fs.Usage()
		return err
	}

	path := opts.path
	if path == "" {
		path, err = defaultCompletionPath(shell)
		if err != nil {
			return err
		}
	}

	if err := writeFileWithDirs(path, []byte(script)); err != nil {
		return err
	}

	fmt.Fprintf(out, "installed completion to %s\n", path)
	return nil
}

func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletion(), nil
	case "zsh":
		return zshCompletion(), nil
	case "fish":
		return fishCompletion(), nil
	default:
		return "", fmt.Errorf("%w: unknown shell %q (want: bash, zsh, fish)", errUsage, shell)
	}
}

func bashCompletion() string {
	return `_fantastical_completions() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  local cmds="parse show applescript as completion help version"
  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
    return 0
  fi

  case "${COMP_WORDS[1]}" in
    parse)
      local flags="--note -n --calendar --calendarName --add --open --print --copy --json --plain --dry-run --verbose --stdin --param --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    show)
      local flags="--open --print --copy --json --plain --dry-run --verbose --param --help"
      local subs="mini calendar day week month agenda set"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    applescript|as)
      local flags="--add --run --print --dry-run --verbose --stdin --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    completion)
      local subs="install bash zsh fish"
      COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
      ;;
  esac
}

complete -F _fantastical_completions fantastical`
}

func zshCompletion() string {
	return `#compdef fantastical

_fantastical() {
  local -a commands
  commands=(
    'parse:Build x-fantastical3://parse URL'
    'show:Build x-fantastical3://show URL'
    'applescript:Run Fantastical AppleScript'
    'completion:Generate shell completion'
    'help:Show help for a command'
    'version:Print version information'
  )

  _arguments -C \
    '1:command:->cmds' \
    '*::arg:->args'

  case $state in
    cmds)
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        parse)
          _arguments '*:sentence:' \
            '--note[Optional note]' \
            '-n[Optional note]' \
            '--calendar[Calendar name]' \
            '--calendarName[Calendar name]' \
            '--add[Add immediately]' \
            '--open[Open URL]' \
            '--print[Print URL]' \
            '--copy[Copy URL]' \
            '--json[JSON output]' \
            '--plain[Plain output]' \
            '--dry-run[Preview only]' \
            '--verbose[Verbose output]' \
            '--stdin[Read from stdin]' \
            '--param[Extra query param]'
          ;;
        show)
          _arguments '1:target:(mini calendar day week month agenda set)' '*:date or name:' \
            '--open[Open URL]' \
            '--print[Print URL]' \
            '--copy[Copy URL]' \
            '--json[JSON output]' \
            '--plain[Plain output]' \
            '--dry-run[Preview only]' \
            '--verbose[Verbose output]' \
            '--param[Extra query param]'
          ;;
        applescript|as)
          _arguments '*:sentence:' \
            '--add[Add immediately]' \
            '--run[Run osascript]' \
            '--print[Print script]' \
            '--dry-run[Preview only]' \
            '--verbose[Verbose output]' \
            '--stdin[Read from stdin]'
          ;;
        completion)
          _arguments '1:sub:(install bash zsh fish)'
          ;;
        help)
          _arguments '1:command:(parse show applescript completion help version)'
          ;;
      esac
      ;;
  esac
}

_fantastical "$@"`
}

func fishCompletion() string {
	return `complete -c fantastical -f
complete -c fantastical -n '__fish_use_subcommand' -a 'parse show applescript completion help version' -d 'fantastical command'

complete -c fantastical -n '__fish_seen_subcommand_from parse' -l note -d 'Optional note'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -s n -d 'Optional note'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l calendar -d 'Calendar name'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l calendarName -d 'Calendar name'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l add -d 'Add immediately'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l open -d 'Open URL'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l print -d 'Print URL'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l copy -d 'Copy URL'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l plain -d 'Plain output'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l dry-run -d 'Preview only'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l stdin -d 'Read from stdin'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l param -d 'Extra query param'

complete -c fantastical -n '__fish_seen_subcommand_from show' -a 'mini calendar day week month agenda set' -d 'Show target'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l open -d 'Open URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l print -d 'Print URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l copy -d 'Copy URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l plain -d 'Plain output'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l dry-run -d 'Preview only'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l param -d 'Extra query param'

complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l add -d 'Add immediately'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l run -d 'Run osascript'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l print -d 'Print script'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l dry-run -d 'Preview only'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l stdin -d 'Read from stdin'

complete -c fantastical -n '__fish_seen_subcommand_from completion' -a 'install bash zsh fish' -d 'Shell'
complete -c fantastical -n '__fish_seen_subcommand_from help' -a 'parse show applescript completion help version' -d 'Command'`
}

func buildParseURL(sentence, note, calendar string, add bool, extra url.Values) string {
	q := url.Values{}
	q.Set("s", sentence)
	if strings.TrimSpace(note) != "" {
		q.Set("n", note)
	}
	if strings.TrimSpace(calendar) != "" {
		q.Set("calendarName", calendar)
	}
	if add {
		q.Set("add", "1")
	}
	for key, vals := range extra {
		for _, v := range vals {
			q.Add(key, v)
		}
	}
	return fantasticalScheme + "parse?" + encodeQuery(q)
}

// encodeQuery is like url.Values.Encode(), but uses %20 for spaces instead of '+'.
// Some custom URL handlers are picky; %20 tends to be accepted everywhere.
func encodeQuery(v url.Values) string {
	enc := v.Encode()
	return strings.ReplaceAll(enc, "+", "%20")
}

func openURL(u string, out, errOut io.Writer) error {
	cmdName, cmdArgs, err := openCommand(u)
	if err != nil {
		return err
	}

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func openCommand(u string) (string, []string, error) {
	if override := strings.TrimSpace(os.Getenv("FANTASTICAL_OPEN_COMMAND")); override != "" {
		parts := strings.Fields(override)
		if len(parts) == 0 {
			return "", nil, errors.New("FANTASTICAL_OPEN_COMMAND is empty")
		}
		return parts[0], append(parts[1:], u), nil
	}

	return "open", []string{u}, nil
}

func osascriptCommand() string {
	if override := strings.TrimSpace(os.Getenv("FANTASTICAL_OSASCRIPT_COMMAND")); override != "" {
		parts := strings.Fields(override)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return "osascript"
}

func copyToClipboard(text string) error {
	path, err := exec.LookPath("pbcopy")
	if err != nil {
		return errors.New("pbcopy not found (install Xcode command line tools or use --print)")
	}
	cmd := exec.Command(path)
	cmd.Stdin = strings.NewReader(text)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func parseDateArg(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("%w: empty date", errUsage)
	}

	now := time.Now()
	// normalize to local midnight for relative dates
	midnight := func(t time.Time) time.Time {
		loc := t.Location()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	}

	switch strings.ToLower(s) {
	case "today":
		return midnight(now), nil
	case "tomorrow":
		return midnight(now.AddDate(0, 0, 1)), nil
	case "yesterday":
		return midnight(now.AddDate(0, 0, -1)), nil
	default:
		// Expect yyyy-mm-dd
		t, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: invalid date %q; want yyyy-mm-dd (e.g. 2026-01-03) or today/tomorrow/yesterday", errUsage, s)
		}
		return t, nil
	}
}

func readSentence(args []string, fromStdin bool, in io.Reader) (string, error) {
	if fromStdin {
		if len(args) > 0 {
			return "", fmt.Errorf("%w: cannot use args with --stdin", errUsage)
		}
		data, err := io.ReadAll(in)
		if err != nil {
			return "", fmt.Errorf("%w: read stdin: %v", errUsage, err)
		}
		sentence := strings.TrimSpace(string(data))
		if sentence == "" {
			return "", fmt.Errorf("%w: empty stdin", errUsage)
		}
		return sentence, nil
	}

	sentence := strings.TrimSpace(strings.Join(args, " "))
	if sentence == "" {
		return "", fmt.Errorf("%w: missing <sentence...>", errUsage)
	}
	return sentence, nil
}

func parseParams(params []string) (url.Values, error) {
	q := url.Values{}
	for _, raw := range params {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("%w: invalid param %q (want key=value)", errUsage, raw)
		}
		key := strings.TrimSpace(parts[0])
		value := ""
		if len(parts) == 2 {
			value = strings.TrimSpace(parts[1])
		}
		q.Add(key, value)
	}
	return q, nil
}

func writeJSON(w io.Writer, payload any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(payload)
}

func logVerbose(w io.Writer, verbose bool, format string, args ...any) {
	if !verbose {
		return
	}
	fmt.Fprintf(w, "[fantastical] "+format+"\n", args...)
}

func defaultCompletionPath(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	switch shell {
	case "bash":
		if _, err := os.Stat("/opt/homebrew"); err == nil {
			return "/opt/homebrew/etc/bash_completion.d/fantastical", nil
		}
		return "/usr/local/etc/bash_completion.d/fantastical", nil
	case "zsh":
		return home + "/.zsh/completions/_fantastical", nil
	case "fish":
		return home + "/.config/fish/completions/fantastical.fish", nil
	default:
		return "", fmt.Errorf("%w: unknown shell %q (want: bash, zsh, fish)", errUsage, shell)
	}
}

func writeFileWithDirs(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
