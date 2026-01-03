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
//	./fantastical parse --add --calendar "Work" --note "Alarm" "Wake up at 8am"
//	./fantastical show mini today
//	./fantastical show calendar 2026-01-03
//	./fantastical show set "My Calendar Set"
//	./fantastical applescript --add "Wake up at 8am"
package main

import (
	"bytes"
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
	case "validate":
		err = cmdValidate(args[2:], in, out, errOut)
	case "doctor":
		err = cmdDoctor(args[2:], out, errOut)
	case "eventkit":
		err = cmdEventKit(args[2:], out, errOut)
	case "greta":
		err = cmdGreta(args[2:], out, errOut)
	case "explain":
		err = cmdExplain(args[2:], out, errOut)
	case "man":
		err = cmdMan(args[2:], out, errOut)
	case "completion":
		err = cmdCompletion(args[2:], out, errOut)
	case "help", "-h", "--help":
		err = cmdHelp(args[2:], out, errOut)
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
  validate     Validate parse/show input and print the URL
  doctor       Check Fantastical + macOS integration status
  eventkit     List calendars or events via EventKit (system Calendar access)
  greta        Machine-readable CLI spec for agents
  explain      Human-readable command walkthrough
  man          Manual page output (markdown or json)
  completion   Print or install shell completion (bash|zsh|fish)
  help         Show help for a command
  version      Print version information

NOTES
  - macOS only (Fantastical is a macOS app).
  - --open defaults to true (uses "open <url>").
  - Use --json for machine-readable output; use --plain for stable text output.
  - For parse/applescript, put flags before the sentence or use -- to separate.
  - eventkit commands require Calendar access; macOS will prompt on first use.

EXAMPLES
  fantastical parse --add --calendar "Work" --note "Alarm" "Wake up at 8am"
  fantastical parse --print "Dinner with Sam tomorrow 7pm"
  fantastical parse --stdin --json < input.txt
  fantastical show mini today
  fantastical show month 2026-01-03
  fantastical show set "My Calendar Set"
  fantastical applescript --add "Wake up at 8am"
  fantastical eventkit status --json
  fantastical eventkit calendars --json
  fantastical eventkit events --next-week --calendar "Work"
  fantastical greta --format json
  fantastical help --json parse
  fantastical explain parse
  fantastical man --format markdown
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
	case "validate":
		validateUsage(w)
		return nil
	case "doctor":
		doctorUsage(w)
		return nil
	case "eventkit":
		eventKitUsage(w)
		return nil
	case "greta":
		gretaUsage(w)
		return nil
	case "explain":
		explainUsage(w)
		return nil
	case "man":
		manUsage(w)
		return nil
	default:
		return fmt.Errorf("%w: unknown help topic %q", errUsage, cmd)
	}
}

type helpOptions struct {
	json bool
}

func cmdHelp(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("help", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := helpOptions{}
	fs.BoolVar(&opts.json, "json", false, "Print machine-readable JSON help")

	fs.Usage = func() {
		fmt.Fprint(errOut, "USAGE:\n  fantastical help [--json] [command]\n")
		fmt.Fprintln(errOut, "\nEXAMPLE:\n  fantastical help --json parse")
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	cmd := ""
	if fs.NArg() > 0 {
		cmd = fs.Arg(0)
	}

	if opts.json {
		spec, err := helpSpec(cmd)
		if err != nil {
			return err
		}
		return writeJSON(out, spec)
	}

	if cmd == "" {
		usage(out)
		return nil
	}

	return printSubcommandHelp(cmd, out)
}

func helpSpec(command string) (any, error) {
	spec := gretaSpec("v1")
	if strings.TrimSpace(command) == "" {
		return spec, nil
	}

	cmd := strings.ToLower(strings.TrimSpace(command))
	commands, ok := spec["commands"].([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid greta spec")
	}
	for _, entry := range commands {
		if name, _ := entry["name"].(string); strings.EqualFold(name, cmd) {
			return map[string]any{
				"schemaVersion": spec["schemaVersion"],
				"command":       entry,
			}, nil
		}
	}
	return nil, fmt.Errorf("%w: unknown help topic %q", errUsage, command)
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
	timezone string
	config   string
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
	fs.StringVar(&opts.timezone, "timezone", "", "Timezone to pass as tz=... (IANA name)")
	fs.StringVar(&opts.config, "config", "", "Config file path (overrides default user config)")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical parse [flags] <sentence...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical parse --add --calendar Work --note \"Alarm\" \"Wake up at 8am\"")
		fmt.Fprintln(w, "NOTE:\n  Put flags before the sentence, or use -- to separate flags from the sentence.")
	}

	return fs, &opts
}

func cmdParse(args []string, in io.Reader, out, errOut io.Writer) error {
	configPath, err := extractConfigPath(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfigWithPath(configPath)
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
	if strings.TrimSpace(opts.timezone) != "" {
		extraParams.Set("tz", strings.TrimSpace(opts.timezone))
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
	view   string
	set    string
	tz     string
	config string
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
	fs.StringVar(&opts.view, "view", "", "View name (e.g., mini, calendar, day, week, month, agenda)")
	fs.StringVar(&opts.set, "calendar-set", "", "Calendar set name (equivalent to: show set <name>)")
	fs.StringVar(&opts.tz, "timezone", "", "Timezone to pass as tz=... (IANA name)")
	fs.StringVar(&opts.config, "config", "", "Config file path (overrides default user config)")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical show [flags] <view> [yyyy-mm-dd|today|tomorrow|yesterday]\n  fantastical show [flags] --view <view> [date]\n  fantastical show [flags] set <calendar-set-name...>\n  fantastical show [flags] --calendar-set <name>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical show mini today\n  fantastical show --view month 2026-01-03\n  fantastical show --calendar-set \"My Calendar Set\"")
	}

	return fs, &opts
}

func cmdShow(args []string, out, errOut io.Writer) error {
	configPath, err := extractConfigPath(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfigWithPath(configPath)
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
	sub := strings.ToLower(strings.TrimSpace(opts.view))

	extraParams, err := parseParams(opts.params)
	if err != nil {
		fs.Usage()
		return err
	}
	if strings.TrimSpace(opts.tz) != "" {
		extraParams.Set("tz", strings.TrimSpace(opts.tz))
	}

	var u string
	if strings.TrimSpace(opts.set) != "" {
		if len(rest) > 0 || sub != "" {
			fs.Usage()
			return fmt.Errorf("%w: cannot combine --calendar-set with positional view arguments", errUsage)
		}
		name := strings.TrimSpace(opts.set)
		q := url.Values{}
		q.Set("name", name)
		for key, vals := range extraParams {
			for _, v := range vals {
				q.Add(key, v)
			}
		}
		u = fantasticalScheme + "show/set?" + encodeQuery(q)
	} else {
		if sub == "" {
			if len(rest) < 1 {
				fs.Usage()
				return fmt.Errorf("%w: missing view", errUsage)
			}
			sub = strings.ToLower(rest[0])
			rest = rest[1:]
		}
		if len(rest) > 1 {
			fs.Usage()
			return fmt.Errorf("%w: too many args for %q; expected: fantastical show %s [date]", errUsage, sub, sub)
		}
		u = fantasticalScheme + "show/" + sub
		if len(rest) == 1 {
			d, err := parseDateArg(rest[0])
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
	config  string
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
	fs.StringVar(&opts.config, "config", "", "Config file path (overrides default user config)")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical applescript|as [flags] <sentence...>\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical applescript --add \"Wake up at 8am\"")
		fmt.Fprintln(w, "NOTE:\n  Put flags before the sentence, or use -- to separate flags from the sentence.")
	}

	return fs, &opts
}

func cmdAppleScript(args []string, in io.Reader, out, errOut io.Writer) error {
	configPath, err := extractConfigPath(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfigWithPath(configPath)
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

func validateUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical validate [--json] parse [flags] <sentence...>\n  fantastical validate [--json] show [flags] <view> [date]\n")
	fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical validate --json parse \"Dinner at 7\"\n  fantastical validate show month 2026-01-03")
}

func cmdValidate(args []string, in io.Reader, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	jsonOut := false
	fs.BoolVar(&jsonOut, "json", false, "Print machine-readable JSON validation result")

	fs.Usage = func() {
		validateUsage(errOut)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	if fs.NArg() < 1 {
		validateUsage(errOut)
		return fmt.Errorf("%w: missing validate target (parse|show)", errUsage)
	}

	sub := strings.ToLower(fs.Arg(0))
	rest := fs.Args()[1:]
	rest = append([]string{"--dry-run", "--print"}, rest...)

	runValidate := func(fn func([]string, io.Reader, io.Writer, io.Writer) error) error {
		if !jsonOut {
			return fn(rest, in, out, errOut)
		}

		var buf bytes.Buffer
		err := fn(rest, in, &buf, errOut)
		if err != nil {
			payload := map[string]any{
				"ok":     false,
				"target": sub,
				"error":  err.Error(),
			}
			return writeJSON(out, payload)
		}

		payload := map[string]any{
			"ok":     true,
			"target": sub,
			"output": strings.TrimSpace(buf.String()),
		}
		return writeJSON(out, payload)
	}

	switch sub {
	case "parse":
		return runValidate(func(args []string, in io.Reader, out io.Writer, errOut io.Writer) error {
			return cmdParse(args, in, out, errOut)
		})
	case "show":
		return runValidate(func(args []string, in io.Reader, out io.Writer, errOut io.Writer) error {
			return cmdShow(args, out, errOut)
		})
	default:
		validateUsage(errOut)
		return fmt.Errorf("%w: unknown validate target %q (want: parse, show)", errUsage, sub)
	}
}

type doctorOptions struct {
	json    bool
	verbose bool
	skipApp bool
}

func doctorUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical doctor [--json] [--skip-app]\n")
	fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical doctor --json")
}

func cmdDoctor(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := doctorOptions{}
	fs.BoolVar(&opts.json, "json", false, "Print machine-readable JSON output")
	fs.BoolVar(&opts.verbose, "verbose", false, "Verbose output to stderr")
	fs.BoolVar(&opts.skipApp, "skip-app", false, "Skip Fantastical app lookup")

	fs.Usage = func() {
		doctorUsage(errOut)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	osascriptPath, osascriptErr := exec.LookPath("osascript")
	pbcopyPath, pbcopyErr := exec.LookPath("pbcopy")
	appErr := error(nil)
	if !opts.skipApp {
		appErr = exec.Command("open", "-Ra", "Fantastical").Run()
	}

	if opts.json {
		payload := map[string]any{
			"osascript": map[string]any{
				"ok":   osascriptErr == nil,
				"path": osascriptPath,
			},
			"pbcopy": map[string]any{
				"ok":   pbcopyErr == nil,
				"path": pbcopyPath,
			},
			"fantastical_app": map[string]any{
				"ok":    appErr == nil,
				"check": !opts.skipApp,
			},
			"permissions": "Grant Terminal Automation permission if AppleScript prompts or fails.",
		}
		if err := writeJSON(out, payload); err != nil {
			return err
		}
	} else {
		if appErr == nil {
			fmt.Fprintln(out, "Fantastical app: ok")
		} else if opts.skipApp {
			fmt.Fprintln(out, "Fantastical app: skipped")
		} else {
			fmt.Fprintln(out, "Fantastical app: not found (install from the App Store)")
		}

		if osascriptErr == nil {
			fmt.Fprintln(out, "osascript: ok")
		} else {
			fmt.Fprintln(out, "osascript: missing (macOS scripting not available)")
		}

		if pbcopyErr == nil {
			fmt.Fprintln(out, "pbcopy: ok")
		} else {
			fmt.Fprintln(out, "pbcopy: missing (install Xcode command line tools)")
		}

		fmt.Fprintln(out, "Automation permissions: grant Terminal access if AppleScript fails.")
	}

	if appErr != nil && !opts.skipApp {
		return fmt.Errorf("Fantastical app not found")
	}
	if osascriptErr != nil {
		return fmt.Errorf("osascript not available")
	}

	logVerbose(errOut, opts.verbose, "doctor completed")
	return nil
}

type gretaOptions struct {
	format       string
	schema       string
	examples     bool
	capabilities bool
}

func gretaUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical greta [--format json|markdown] [--schema v1] [--examples] [--capabilities]\n")
	fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical greta --format json\n  fantastical greta --examples\n  fantastical greta --capabilities --format json")
}

func cmdGreta(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("greta", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := gretaOptions{format: "json", schema: "v1"}
	fs.StringVar(&opts.format, "format", opts.format, "Output format: json or markdown")
	fs.StringVar(&opts.schema, "schema", opts.schema, "Schema version (v1)")
	fs.BoolVar(&opts.examples, "examples", false, "Output curated examples only")
	fs.BoolVar(&opts.capabilities, "capabilities", false, "Output capability summary only")

	fs.Usage = func() {
		gretaUsage(errOut)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	format := strings.ToLower(strings.TrimSpace(opts.format))
	if strings.TrimSpace(opts.schema) == "" {
		opts.schema = "v1"
	}
	if opts.schema != "v1" {
		fs.Usage()
		return fmt.Errorf("%w: unknown schema %q (want: v1)", errUsage, opts.schema)
	}

	if opts.examples && opts.capabilities {
		fs.Usage()
		return fmt.Errorf("%w: --examples and --capabilities are mutually exclusive", errUsage)
	}

	if opts.examples {
		if format == "markdown" {
			fmt.Fprintln(out, gretaExamplesMarkdown())
			return nil
		}
		return writeJSON(out, gretaExamples(opts.schema))
	}
	if opts.capabilities {
		if format == "markdown" {
			fmt.Fprintln(out, gretaCapabilitiesMarkdown())
			return nil
		}
		return writeJSON(out, gretaCapabilities(opts.schema))
	}

	switch format {
	case "json":
		return writeJSON(out, gretaSpec(opts.schema))
	case "markdown":
		fmt.Fprintln(out, gretaMarkdown())
		return nil
	default:
		fs.Usage()
		return fmt.Errorf("%w: unknown format %q (want: json, markdown)", errUsage, opts.format)
	}
}

func gretaSpec(schema string) map[string]any {
	return map[string]any{
		"schemaVersion": schema,
		"name":          appName,
		"description":   "CLI for Fantastical URL handler and AppleScript integration (macOS only)",
		"usage":         "fantastical [--version] <command> [flags] [args]",
		"notes": []string{
			"For parse/applescript, put flags before the sentence or use -- to separate.",
			"EventKit commands require Calendar access and compile a helper with swiftc on first use.",
		},
		"commands": []map[string]any{
			{
				"name":        "parse",
				"description": "Build x-fantastical3://parse URL",
				"args":        "<sentence...>",
				"flags": []string{
					"--note, -n",
					"--calendar, --calendarName",
					"--add",
					"--open",
					"--print",
					"--copy",
					"--json",
					"--plain",
					"--dry-run",
					"--verbose",
					"--stdin",
					"--param key=value",
					"--timezone IANA",
					"--config path",
				},
			},
			{
				"name":        "show",
				"description": "Build x-fantastical3://show URL",
				"args":        "<view> [date] | set <calendar-set-name>",
				"flags": []string{
					"--view",
					"--calendar-set",
					"--open",
					"--print",
					"--copy",
					"--json",
					"--plain",
					"--dry-run",
					"--verbose",
					"--param key=value",
					"--timezone IANA",
					"--config path",
				},
			},
			{
				"name":        "applescript",
				"description": "Run Fantastical AppleScript parse sentence",
				"args":        "<sentence...>",
				"flags": []string{
					"--add",
					"--run",
					"--print",
					"--dry-run",
					"--verbose",
					"--stdin",
					"--config path",
				},
			},
			{
				"name":        "validate",
				"description": "Validate parse/show input and print URL",
				"args":        "parse|show ...",
			},
			{
				"name":        "doctor",
				"description": "Check Fantastical + macOS integration status",
				"flags": []string{
					"--json",
					"--skip-app",
					"--verbose",
				},
			},
			{
				"name":        "eventkit",
				"description": "List calendars or events via EventKit",
				"args":        "status|calendars|events [flags]",
				"flags": []string{
					"--format plain|json|table",
					"--json",
					"--plain",
					"--no-input",
					"--calendar",
					"--calendar-id",
					"--from",
					"--to",
					"--days",
					"--today",
					"--tomorrow",
					"--this-week",
					"--next-week",
					"--limit",
					"--include-all-day",
					"--include-declined",
					"--sort start|end|title|calendar",
					"--tz IANA",
					"--query text",
				},
			},
			{
				"name":        "greta",
				"description": "Machine-readable CLI spec for agents",
				"flags": []string{
					"--format json|markdown",
					"--schema v1",
					"--examples",
					"--capabilities",
				},
			},
			{
				"name":        "explain",
				"description": "Human-readable command walkthrough",
				"args":        "<command>",
			},
			{
				"name":        "man",
				"description": "Manual page output (markdown or json)",
				"flags": []string{
					"--format markdown|json",
				},
			},
			{
				"name":        "help",
				"description": "Show help for a command",
				"flags": []string{
					"--json",
				},
			},
			{
				"name":        "completion",
				"description": "Print or install/uninstall shell completions",
				"args":        "[bash|zsh|fish]",
			},
		},
		"output": map[string]any{
			"stdout": "URLs, JSON output, scripts, or diagnostics",
			"stderr": "Errors and verbose logs",
		},
		"config": map[string]any{
			"user":    "~/.config/fantastical/config.json",
			"project": ".fantastical.json",
			"env":     "FANTASTICAL_CONFIG overrides user config path",
			"order":   "flags > env > project config > user config",
		},
		"env": []string{
			"FANTASTICAL_DEFAULT_OPEN",
			"FANTASTICAL_DEFAULT_PRINT",
			"FANTASTICAL_DEFAULT_COPY",
			"FANTASTICAL_DEFAULT_JSON",
			"FANTASTICAL_DEFAULT_PLAIN",
			"FANTASTICAL_DRY_RUN",
			"FANTASTICAL_VERBOSE",
			"FANTASTICAL_DEFAULT_CALENDAR",
			"FANTASTICAL_DEFAULT_NOTE",
			"FANTASTICAL_DEFAULT_ADD",
			"FANTASTICAL_APPLESCRIPT_ADD",
			"FANTASTICAL_APPLESCRIPT_RUN",
			"FANTASTICAL_APPLESCRIPT_PRINT",
			"FANTASTICAL_EVENTKIT_HELPER",
		},
		"exit_codes": map[string]int{
			"success": 0,
			"usage":   2,
			"error":   1,
		},
	}
}

func gretaMarkdown() string {
	return `# fantastical CLI spec

- Name: fantastical
- Usage: fantastical [--version] <command> [flags] [args]
- macOS only
- Note: for parse/applescript, put flags before the sentence or use -- to separate.
- Note: eventkit commands request Calendar access on first use.
- Note: eventkit builds a small Swift helper (swiftc) on first use.

## Commands
- parse: build x-fantastical3://parse URL
- show: build x-fantastical3://show URL
- applescript: run Fantastical AppleScript parse sentence
- validate: validate parse/show input and print URL
- doctor: check Fantastical + macOS integration status
- eventkit: list calendars or events via EventKit
- greta: machine-readable CLI spec for agents
- explain: human-readable command walkthrough
- man: manual page output
- completion: print/install/uninstall shell completions

## Config
- User: ~/.config/fantastical/config.json
- Project: .fantastical.json
- Env override: FANTASTICAL_CONFIG
- Precedence: flags > env > project config > user config
`
}

func gretaExamples(schema string) map[string]any {
	return map[string]any{
		"schemaVersion": schema,
		"examples": []map[string]any{
			{
				"description": "Create an event with a calendar and note",
				"command":     `fantastical parse --add --calendar "Work" --note "Alarm" "Wake up at 8am"`,
			},
			{
				"description": "Build a URL without opening (JSON output)",
				"command":     `fantastical parse --json "Dinner tomorrow 7pm"`,
			},
			{
				"description": "Show month view on a specific date",
				"command":     `fantastical show --view month 2026-01-03`,
			},
			{
				"description": "Show a calendar set",
				"command":     `fantastical show --calendar-set "My Calendar Set"`,
			},
			{
				"description": "Validate a parse command",
				"command":     `fantastical validate --json parse "Dinner at 7"`,
			},
			{
				"description": "Check installation and permissions",
				"command":     `fantastical doctor --json`,
			},
			{
				"description": "List calendars via EventKit",
				"command":     `fantastical eventkit calendars --json`,
			},
			{
				"description": "Check EventKit authorization status",
				"command":     `fantastical eventkit status --json`,
			},
			{
				"description": "List events for a date range via EventKit",
				"command":     `fantastical eventkit events --next-week --calendar "Work"`,
			},
		},
	}
}

func gretaExamplesMarkdown() string {
	return `# fantastical examples

- Create an event with a calendar and note:
  fantastical parse --add --calendar "Work" --note "Alarm" "Wake up at 8am"
- Build a URL without opening (JSON output):
  fantastical parse --json "Dinner tomorrow 7pm"
- Show month view on a specific date:
  fantastical show --view month 2026-01-03
- Show a calendar set:
  fantastical show --calendar-set "My Calendar Set"
- Validate a parse command:
  fantastical validate --json parse "Dinner at 7"
- Check installation and permissions:
  fantastical doctor --json
- List calendars via EventKit:
  fantastical eventkit calendars --json
- Check EventKit authorization status:
  fantastical eventkit status --json
- List events for a date range via EventKit:
  fantastical eventkit events --next-week --calendar "Work"
`
}

func gretaCapabilities(schema string) map[string]any {
	return map[string]any{
		"schemaVersion": schema,
		"platform":      "macOS",
		"views": []string{
			"mini", "calendar", "day", "week", "month", "agenda", "set",
		},
		"outputModes": []string{
			"plain", "json",
		},
		"features": []string{
			"stdin",
			"config",
			"completion",
			"validate",
			"doctor",
			"eventkit",
			"greta",
			"explain",
			"man",
		},
	}
}

func gretaCapabilitiesMarkdown() string {
	return `# fantastical capabilities

- Platform: macOS
- Views: mini, calendar, day, week, month, agenda, set
- Output modes: plain, json
- Features: stdin, config, completion, validate, doctor, eventkit, greta, explain, man
`
}

func explainUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical explain <command>\n")
	fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical explain parse")
}

func cmdExplain(args []string, out, errOut io.Writer) error {
	if len(args) < 1 {
		explainUsage(errOut)
		return fmt.Errorf("%w: missing command", errUsage)
	}

	text, err := explainText(args[0])
	if err != nil {
		explainUsage(errOut)
		return err
	}

	fmt.Fprintln(out, text)
	return nil
}

func explainText(command string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "parse":
		return `parse builds an x-fantastical3://parse URL from a natural language sentence.

Common usage:
  fantastical parse --add --calendar "Work" "Meet Sam tomorrow 3pm"

Flags:
  --note, --calendar, --add control Fantastical's parse behavior.
  Put flags before the sentence, or use -- to separate flags from the sentence.
  --param key=value lets you pass extra Fantastical query params.
  --timezone sets tz=... for the URL.
  --json outputs machine-readable JSON with the URL.
  --dry-run disables open/copy side effects.`, nil
	case "show":
		return `show builds x-fantastical3://show URLs for views or calendar sets.

Examples:
  fantastical show mini today
  fantastical show --view month 2026-01-03
  fantastical show --calendar-set "My Calendar Set"

Use --timezone to set tz=... and --param to pass extra query params.`, nil
	case "applescript":
		return `applescript sends a sentence to Fantastical via osascript.

Example:
  fantastical applescript --add "Wake up at 8am"

Use --print to see the script and --dry-run to avoid execution.`, nil
	case "validate":
		return `validate checks parse/show input and prints the resulting URL.

Example:
  fantastical validate --json parse "Dinner at 7"

Useful for scripting and CI checks.`, nil
	case "doctor":
		return `doctor checks Fantastical app availability and macOS tooling.

Example:
  fantastical doctor --json

If AppleScript fails, grant Terminal Automation permission.`, nil
	case "eventkit":
		return `eventkit lists calendars or events via EventKit (system Calendar access).

Examples:
  fantastical eventkit status --json
  fantastical eventkit calendars --format table
  fantastical eventkit events --next-week --calendar "Work"

Note:
  macOS will prompt for Calendar access on first use. Use --no-input to fail instead of prompting.
  Use --format to select plain/json/table output and --query to filter events.`, nil
	case "greta":
		return `greta outputs a full CLI spec for AI agents.

Examples:
  fantastical greta --format json
  fantastical greta --examples
  fantastical greta --capabilities`, nil
	case "explain":
		return "explain prints a human-readable walkthrough for a command.", nil
	case "man":
		return "man outputs the full manual in markdown or JSON.", nil
	case "completion":
		return `completion prints or installs shell completion scripts.

Examples:
  fantastical completion zsh
  fantastical completion install zsh
  fantastical completion uninstall zsh`, nil
	case "help":
		return "help shows command usage. Use --json for machine-readable help.", nil
	case "version":
		return "version prints the CLI version.", nil
	default:
		return "", fmt.Errorf("%w: unknown command %q", errUsage, command)
	}
}

type manOptions struct {
	format string
}

func manUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical man [--format markdown|json]\n")
	fmt.Fprintln(w, "\nEXAMPLE:\n  fantastical man --format json")
}

func cmdMan(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("man", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := manOptions{format: "markdown"}
	fs.StringVar(&opts.format, "format", opts.format, "Output format: markdown or json")

	fs.Usage = func() {
		manUsage(errOut)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	format := strings.ToLower(strings.TrimSpace(opts.format))
	switch format {
	case "markdown":
		fmt.Fprintln(out, manMarkdown())
		return nil
	case "json":
		return writeJSON(out, manSpec())
	default:
		fs.Usage()
		return fmt.Errorf("%w: unknown format %q (want: markdown, json)", errUsage, opts.format)
	}
}

func manSpec() map[string]any {
	return map[string]any{
		"name":        appName,
		"synopsis":    "fantastical [--version] <command> [flags] [args]",
		"description": "CLI for Fantastical URL handler and AppleScript integration (macOS only).",
		"commands":    gretaSpec("v1")["commands"],
		"config": map[string]any{
			"user":    "~/.config/fantastical/config.json",
			"project": ".fantastical.json",
			"env":     "FANTASTICAL_CONFIG overrides user config path",
			"order":   "flags > env > project config > user config",
		},
		"exit_codes": map[string]int{
			"success": 0,
			"usage":   2,
			"error":   1,
		},
	}
}

func manMarkdown() string {
	return `# fantastical

## NAME
fantastical — CLI for Fantastical URL handler and AppleScript integration (macOS only)

## SYNOPSIS
fantastical [--version] <command> [flags] [args]

## DESCRIPTION
Use Fantastical's URL handler and AppleScript integration from the command line.

For parse/applescript, put flags before the sentence or use -- to separate.

EventKit commands require Calendar access; macOS will prompt on first use.
EventKit commands compile a small Swift helper (swiftc) on first use.

## COMMANDS
See fantastical help or fantastical greta --format json.

## CONFIG
User: ~/.config/fantastical/config.json
Project: .fantastical.json
Precedence: flags > env > project config > user config

## EXIT CODES
0 success, 1 error, 2 usage
`
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
		fmt.Fprint(w, "USAGE:\n  fantastical completion [bash|zsh|fish]\n  fantastical completion install [--path <path>] [bash|zsh|fish]\n  fantastical completion uninstall [--path <path>] [bash|zsh|fish]\n")
		fmt.Fprintln(w, "\nEXAMPLES:\n  fantastical completion zsh\n  fantastical completion install fish\n  fantastical completion uninstall zsh")
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
	if rest[0] == "uninstall" {
		return cmdCompletionUninstall(rest[1:], out, errOut)
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

func extractConfigPath(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--config=") {
			value := strings.TrimPrefix(arg, "--config=")
			if strings.TrimSpace(value) == "" {
				return "", fmt.Errorf("%w: --config requires a value", errUsage)
			}
			return value, nil
		}
		if arg == "--config" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%w: --config requires a value", errUsage)
			}
			return args[i+1], nil
		}
	}
	return "", nil
}

func cmdCompletionUninstall(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("completion uninstall", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := completionInstallOptions{}
	fs.StringVar(&opts.path, "path", "", "Completion path (defaults to a user-local location)")

	fs.Usage = func() {
		fmt.Fprint(errOut, "USAGE:\n  fantastical completion uninstall [--path <path>] [bash|zsh|fish]\n")
		fmt.Fprintln(errOut, "\nEXAMPLE:\n  fantastical completion uninstall zsh")
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
	path := opts.path
	if path == "" {
		var err error
		path, err = defaultCompletionPath(shell)
		if err != nil {
			return err
		}
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("completion not found at %s", path)
		}
		return fmt.Errorf("remove %s: %w", path, err)
	}

	fmt.Fprintf(out, "removed completion from %s\n", path)
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

  local cmds="parse show applescript validate doctor eventkit greta explain man as completion help version"
  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
    return 0
  fi

  case "${COMP_WORDS[1]}" in
    parse)
      local flags="--note -n --calendar --calendarName --add --open --print --copy --json --plain --dry-run --verbose --stdin --param --timezone --config --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    show)
      local flags="--open --print --copy --json --plain --dry-run --verbose --param --view --calendar-set --timezone --config --help"
      local subs="mini calendar day week month agenda set"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    applescript|as)
      local flags="--add --run --print --dry-run --verbose --stdin --config --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    validate)
      local flags="--json --help"
      local subs="parse show"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    doctor)
      local flags="--json --skip-app --verbose --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    eventkit)
      local subs="status calendars events"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
        return 0
      fi
      local sub="${COMP_WORDS[2]}"
      if [[ "$sub" == "status" ]]; then
        local flags="--format --json --plain --verbose --help"
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
      fi
      if [[ "$sub" == "calendars" ]]; then
        local flags="--format --json --plain --no-input --verbose --help"
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
      fi
      if [[ "$sub" == "events" ]]; then
        local flags="--format --json --plain --no-input --verbose --calendar --calendar-id --from --to --days --today --tomorrow --this-week --next-week --limit --include-all-day --include-declined --sort --tz --query --help"
        COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
      ;;
    greta)
      local flags="--format --schema --examples --capabilities --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    explain)
      COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
      ;;
    man)
      local flags="--format --help"
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
      ;;
    completion)
      local subs="install uninstall bash zsh fish"
      COMPREPLY=( $(compgen -W "$subs" -- "$cur") )
      ;;
    help)
      local flags="--json --help"
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(compgen -W "$cmds" -- "$cur") )
        return 0
      fi
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
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
    'validate:Validate input and print URL'
    'doctor:Check Fantastical integration'
    'eventkit:List calendars or events via EventKit'
    'greta:CLI spec for agents'
    'explain:Human-readable command walkthrough'
    'man:Manual page output'
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
            '--param[Extra query param]' \
            '--timezone[Timezone (tz)]' \
            '--config[Config file path]'
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
            '--param[Extra query param]' \
            '--view[View name]' \
            '--calendar-set[Calendar set]' \
            '--timezone[Timezone (tz)]' \
            '--config[Config file path]'
          ;;
        applescript|as)
          _arguments '*:sentence:' \
            '--add[Add immediately]' \
            '--run[Run osascript]' \
            '--print[Print script]' \
            '--dry-run[Preview only]' \
            '--verbose[Verbose output]' \
            '--stdin[Read from stdin]' \
            '--config[Config file path]'
          ;;
        validate)
          _arguments \
            '--json[JSON output]' \
            '1:target:(parse show)' '*:args:'
          ;;
        doctor)
          _arguments \
            '--json[JSON output]' \
            '--skip-app[Skip app check]' \
            '--verbose[Verbose output]'
          ;;
        eventkit)
          if (( CURRENT == 3 )); then
            _arguments '1:sub:(status calendars events)'
            return
          fi
          case $words[3] in
            status)
              _arguments \
                '--format[Output format (plain|json)]' \
                '--json[JSON output]' \
                '--plain[Plain output]' \
                '--verbose[Verbose output]'
              ;;
            calendars)
              _arguments \
                '--format[Output format (plain|json|table)]' \
                '--json[JSON output]' \
                '--plain[Plain output]' \
                '--no-input[Do not prompt for access]' \
                '--verbose[Verbose output]'
              ;;
            events)
              _arguments \
                '--format[Output format (plain|json|table)]' \
                '--json[JSON output]' \
                '--plain[Plain output]' \
                '--no-input[Do not prompt for access]' \
                '--verbose[Verbose output]' \
                '--calendar[Calendar name]' \
                '--calendar-id[Calendar identifier]' \
                '--from[Start date/time]' \
                '--to[End date/time]' \
                '--days[Days from now]' \
                '--today[Today]' \
                '--tomorrow[Tomorrow]' \
                '--this-week[This week]' \
                '--next-week[Next week]' \
                '--limit[Limit events]' \
                '--include-all-day[Include all-day events]' \
                '--include-declined[Include declined events]' \
                '--sort[Sort order]' \
                '--tz[Timezone]' \
                '--query[Query text]'
              ;;
            *)
              _arguments '1:sub:(status calendars events)'
              ;;
          esac
          ;;
        greta)
          _arguments \
            '--format[Output format (json|markdown)]' \
            '--schema[Schema version (v1)]' \
            '--examples[Examples only]' \
            '--capabilities[Capabilities only]'
          ;;
        explain)
          _arguments '1:command:(parse show applescript validate doctor eventkit greta completion help version)'
          ;;
        man)
          _arguments '--format[Output format (markdown|json)]'
          ;;
        completion)
          _arguments '1:sub:(install uninstall bash zsh fish)'
          ;;
        help)
          _arguments '--json[JSON output]' '1:command:(parse show applescript validate doctor eventkit greta explain man completion help version)'
          ;;
      esac
      ;;
  esac
}

_fantastical "$@"`
}

func fishCompletion() string {
	return `complete -c fantastical -f
complete -c fantastical -n '__fish_use_subcommand' -a 'parse show applescript validate doctor eventkit greta explain man completion help version' -d 'fantastical command'

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
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l timezone -d 'Timezone (tz)'
complete -c fantastical -n '__fish_seen_subcommand_from parse' -l config -d 'Config file path'

complete -c fantastical -n '__fish_seen_subcommand_from show' -a 'mini calendar day week month agenda set' -d 'Show target'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l open -d 'Open URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l print -d 'Print URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l copy -d 'Copy URL'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l plain -d 'Plain output'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l dry-run -d 'Preview only'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l param -d 'Extra query param'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l view -d 'View name'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l calendar-set -d 'Calendar set'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l timezone -d 'Timezone (tz)'
complete -c fantastical -n '__fish_seen_subcommand_from show' -l config -d 'Config file path'

complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l add -d 'Add immediately'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l run -d 'Run osascript'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l print -d 'Print script'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l dry-run -d 'Preview only'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l stdin -d 'Read from stdin'
complete -c fantastical -n '__fish_seen_subcommand_from applescript' -l config -d 'Config file path'

complete -c fantastical -n '__fish_seen_subcommand_from validate' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from validate' -a 'parse show' -d 'Validate target'
complete -c fantastical -n '__fish_seen_subcommand_from doctor' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from doctor' -l skip-app -d 'Skip app check'
complete -c fantastical -n '__fish_seen_subcommand_from doctor' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -a 'status calendars events' -d 'EventKit target'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l plain -d 'Plain output'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l format -d 'Output format'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l no-input -d 'Do not prompt for access'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l verbose -d 'Verbose output'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l calendar -d 'Calendar name'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l calendar-id -d 'Calendar identifier'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l from -d 'Start date/time'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l to -d 'End date/time'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l days -d 'Days from now'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l today -d 'Today'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l tomorrow -d 'Tomorrow'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l this-week -d 'This week'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l next-week -d 'Next week'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l limit -d 'Limit events'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l include-all-day -d 'Include all-day events'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l include-declined -d 'Include declined events'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l sort -d 'Sort order'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l tz -d 'Timezone'
complete -c fantastical -n '__fish_seen_subcommand_from eventkit' -l query -d 'Query text'
complete -c fantastical -n '__fish_seen_subcommand_from greta' -l format -d 'Format'
complete -c fantastical -n '__fish_seen_subcommand_from greta' -l schema -d 'Schema'
complete -c fantastical -n '__fish_seen_subcommand_from greta' -l examples -d 'Examples only'
complete -c fantastical -n '__fish_seen_subcommand_from greta' -l capabilities -d 'Capabilities only'
complete -c fantastical -n '__fish_seen_subcommand_from explain' -a 'parse show applescript validate doctor eventkit greta completion help version' -d 'Command'
complete -c fantastical -n '__fish_seen_subcommand_from man' -l format -d 'Format'
complete -c fantastical -n '__fish_seen_subcommand_from completion' -a 'install uninstall bash zsh fish' -d 'Shell'
complete -c fantastical -n '__fish_seen_subcommand_from help' -l json -d 'JSON output'
complete -c fantastical -n '__fish_seen_subcommand_from help' -a 'parse show applescript validate doctor eventkit greta explain man completion help version' -d 'Command'`
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
