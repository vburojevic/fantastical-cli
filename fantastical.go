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
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
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
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) < 2 {
		usage(errOut)
		return 2
	}

	cmd := strings.ToLower(args[1])
	var err error

	switch cmd {
	case "parse":
		err = cmdParse(args[2:], out, errOut)
	case "show":
		err = cmdShow(args[2:], out, errOut)
	case "applescript", "as":
		err = cmdAppleScript(args[2:], out, errOut)
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
  completion   Print shell completion (bash|zsh|fish)
  help         Show help for a command
  version      Print version information

NOTES
  - On macOS, --open defaults to true (uses "open <url>").
  - On other OSes, --open defaults to false, so you'll typically use --print.

EXAMPLES
  fantastical parse "Wake up at 8am" --add --calendar "Work" --note "Alarm"
  fantastical parse --print "Dinner with Sam tomorrow 7pm"
  fantastical show mini today
  fantastical show calendar 2026-01-03
  fantastical show set "My Calendar Set"
  fantastical applescript --add "Wake up at 8am"
`)
}

func printSubcommandHelp(cmd string, w io.Writer) error {
	switch strings.ToLower(cmd) {
	case "parse":
		fs, _ := newParseFlagSet(w)
		fs.Usage()
		return nil
	case "show":
		fs, _ := newShowFlagSet(w)
		fs.Usage()
		return nil
	case "applescript", "as":
		fs, _ := newAppleScriptFlagSet(w)
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

type parseOptions struct {
	note     string
	calendar string
	add      bool
	open     bool
	print    bool
	copy     bool
}

func newParseFlagSet(w io.Writer) (*flag.FlagSet, *parseOptions) {
	opts := &parseOptions{
		open: runtime.GOOS == "darwin",
	}

	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // we'll print our own usage on error/help

	fs.StringVar(&opts.note, "note", "", "Optional note (maps to n=...)")
	fs.StringVar(&opts.note, "n", "", "Alias for --note")

	fs.StringVar(&opts.calendar, "calendar", "", "Optional calendar name (maps to calendarName=...)")
	fs.StringVar(&opts.calendar, "calendarName", "", "Alias for --calendar")

	fs.BoolVar(&opts.add, "add", false, "Add immediately without interaction (maps to add=1)")
	fs.BoolVar(&opts.open, "open", opts.open, "Open the generated URL via system opener")
	fs.BoolVar(&opts.print, "print", false, "Print the generated URL to stdout")
	fs.BoolVar(&opts.copy, "copy", false, "Copy the generated URL to clipboard (pbcopy/wl-copy/xclip/clip, if available)")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical parse [flags] <sentence...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical parse \"Wake up at 8am\" --add --calendar Work --note \"Alarm\"")
	}

	return fs, opts
}

func cmdParse(args []string, out, errOut io.Writer) error {
	fs, opts := newParseFlagSet(errOut)

	if err := fs.Parse(args); err != nil {
		// flag package returns flag.ErrHelp if -h/-help is used.
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	sentence := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if sentence == "" {
		fs.Usage()
		return fmt.Errorf("%w: missing <sentence...>", errUsage)
	}

	u := buildParseURL(sentence, opts.note, opts.calendar, opts.add)

	didSomething := false

	if opts.print || (!opts.open && !opts.copy) {
		fmt.Fprintln(out, u)
		didSomething = true
	}
	if opts.copy {
		if err := copyToClipboard(u); err != nil {
			return err
		}
		didSomething = true
	}
	if opts.open {
		if err := openURL(u); err != nil {
			return err
		}
		didSomething = true
	}

	if !didSomething {
		// Shouldn't happen because of the (!open && !copy) print fallback.
		fmt.Fprintln(out, u)
	}

	return nil
}

type showOptions struct {
	open  bool
	print bool
	copy  bool
}

func newShowFlagSet(w io.Writer) (*flag.FlagSet, *showOptions) {
	opts := &showOptions{
		open: runtime.GOOS == "darwin",
	}

	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.BoolVar(&opts.open, "open", opts.open, "Open the generated URL via system opener")
	fs.BoolVar(&opts.print, "print", false, "Print the generated URL to stdout")
	fs.BoolVar(&opts.copy, "copy", false, "Copy the generated URL to clipboard (pbcopy/wl-copy/xclip/clip, if available)")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical show [flags] mini|calendar [yyyy-mm-dd|today|tomorrow|yesterday]\n  fantastical show [flags] set <calendar-set-name...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical show mini today\n  fantastical show calendar 2026-01-03\n  fantastical show set \"My Calendar Set\"")
	}

	return fs, opts
}

func cmdShow(args []string, out, errOut io.Writer) error {
	fs, opts := newShowFlagSet(errOut)

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
		return fmt.Errorf("%w: missing view (mini|calendar|set)", errUsage)
	}

	sub := strings.ToLower(rest[0])

	var u string
	switch sub {
	case "mini", "calendar":
		if len(rest) > 2 {
			fs.Usage()
			return fmt.Errorf("%w: too many args for %q; expected: fantastical show %s [date]", errUsage, sub, sub)
		}
		if len(rest) == 2 {
			d, err := parseDateArg(rest[1])
			if err != nil {
				return err
			}
			u = fantasticalScheme + "show/" + sub + "/" + d.Format("2006-01-02")
		} else {
			u = fantasticalScheme + "show/" + sub
		}

	case "set":
		if len(rest) < 2 {
			fs.Usage()
			return fmt.Errorf("%w: missing calendar set name", errUsage)
		}
		name := strings.TrimSpace(strings.Join(rest[1:], " "))
		q := url.Values{}
		q.Set("name", name)
		u = fantasticalScheme + "show/set?" + encodeQuery(q)

	default:
		fs.Usage()
		return fmt.Errorf("%w: unknown show target %q (want: mini, calendar, set)", errUsage, sub)
	}

	didSomething := false

	if opts.print || (!opts.open && !opts.copy) {
		fmt.Fprintln(out, u)
		didSomething = true
	}
	if opts.copy {
		if err := copyToClipboard(u); err != nil {
			return err
		}
		didSomething = true
	}
	if opts.open {
		if err := openURL(u); err != nil {
			return err
		}
		didSomething = true
	}

	if !didSomething {
		fmt.Fprintln(out, u)
	}

	return nil
}

type appleScriptOptions struct {
	add   bool
	run   bool
	print bool
}

func newAppleScriptFlagSet(w io.Writer) (*flag.FlagSet, *appleScriptOptions) {
	opts := &appleScriptOptions{
		run: runtime.GOOS == "darwin",
	}

	fs := flag.NewFlagSet("applescript", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.BoolVar(&opts.add, "add", false, "Use Fantastical AppleScript 'with add immediately'")
	fs.BoolVar(&opts.run, "run", opts.run, "Run osascript (macOS only)")
	fs.BoolVar(&opts.print, "print", false, "Print the AppleScript instead of (or in addition to) running it")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical applescript|as [flags] <sentence...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical applescript --add \"Wake up at 8am\"")
	}

	return fs, opts
}

func cmdAppleScript(args []string, out, errOut io.Writer) error {
	fs, opts := newAppleScriptFlagSet(errOut)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	sentence := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if sentence == "" {
		fs.Usage()
		return fmt.Errorf("%w: missing <sentence...>", errUsage)
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
		// If they didn't ask to run, printing is enough.
		return nil
	}

	if runtime.GOOS != "darwin" {
		return errors.New("applescript --run is only supported on macOS (osascript); use --print to output the script")
	}

	osascriptArgs := make([]string, 0, len(scriptLines)*2+3)
	for _, line := range scriptLines {
		osascriptArgs = append(osascriptArgs, "-e", line)
	}
	addArg := "0"
	if opts.add {
		addArg = "1"
	}
	osascriptArgs = append(osascriptArgs, "--", sentence, addArg)

	cmd := exec.Command("osascript", osascriptArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type completionOptions struct{}

func newCompletionFlagSet(w io.Writer) (*flag.FlagSet, *completionOptions) {
	opts := &completionOptions{}

	fs := flag.NewFlagSet("completion", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical completion [bash|zsh|fish]\n")
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical completion zsh")
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

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("%w: missing shell name (bash|zsh|fish)", errUsage)
	}

	shell := strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	var script string
	switch shell {
	case "bash":
		script = bashCompletion()
	case "zsh":
		script = zshCompletion()
	case "fish":
		script = fishCompletion()
	default:
		fs.Usage()
		return fmt.Errorf("%w: unknown shell %q (want: bash, zsh, fish)", errUsage, shell)
	}

	fmt.Fprintln(out, script)
	return nil
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
      local flags="--note -n --calendar --calendarName --add --open --print --copy --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    show)
      local flags="--open --print --copy --help"
      local subs="mini calendar set"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    applescript|as)
      local flags="--add --run --print --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
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
            '--copy[Copy URL]'
          ;;
        show)
          _arguments '1:target:(mini calendar set)' '*:date or name:' \
            '--open[Open URL]' \
            '--print[Print URL]' \
            '--copy[Copy URL]'
          ;;
        applescript|as)
          _arguments '*:sentence:' \
            '--add[Add immediately]' \
            '--run[Run osascript]' \
            '--print[Print script]'
          ;;
        completion)
          _arguments '1:shell:(bash zsh fish)'
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

complete -c fantastical -n '__fish_seen_subcommand_from show' -a 'mini calendar set' -d 'Show target'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l open -d 'Open URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l print -d 'Print URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l copy -d 'Copy URL'

complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l add -d 'Add immediately'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l run -d 'Run osascript'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l print -d 'Print script'

complete -c fantastical -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish' -d 'Shell'
complete -c fantastical -n '__fish_seen_subcommand_from help' -a 'parse show applescript completion help version' -d 'Command'`
}

func buildParseURL(sentence, note, calendar string, add bool) string {
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
	return fantasticalScheme + "parse?" + encodeQuery(q)
}

// encodeQuery is like url.Values.Encode(), but uses %20 for spaces instead of '+'.
// Some custom URL handlers are picky; %20 tends to be accepted everywhere.
func encodeQuery(v url.Values) string {
	enc := v.Encode()
	return strings.ReplaceAll(enc, "+", "%20")
}

func openURL(u string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "linux":
		cmd = exec.Command("xdg-open", u)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		return fmt.Errorf("don't know how to open URLs on %s (use --print)", runtime.GOOS)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyToClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "windows":
		cmd := exec.Command("cmd", "/c", "clip")
		cmd.Stdin = strings.NewReader(text)
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "linux":
		// Try Wayland first, then X11.
		if p, _ := exec.LookPath("wl-copy"); p != "" {
			cmd := exec.Command("wl-copy")
			cmd.Stdin = strings.NewReader(text)
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		if p, _ := exec.LookPath("xclip"); p != "" {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		if p, _ := exec.LookPath("xsel"); p != "" {
			cmd := exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(text)
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return errors.New("clipboard tool not found (need wl-copy, xclip, or xsel)")

	default:
		return fmt.Errorf("clipboard copy not supported on %s", runtime.GOOS)
	}
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
