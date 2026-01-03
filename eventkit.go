//go:build darwin
// +build darwin

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type eventKitCalendarsOptions struct {
	format  string
	json    bool
	plain   bool
	verbose bool
	noInput bool
}

type eventKitEventsOptions struct {
	format          string
	json            bool
	plain           bool
	verbose         bool
	noInput         bool
	calendars       stringSlice
	calendarIDs     stringSlice
	from            string
	to              string
	days            int
	today           bool
	tomorrow        bool
	thisWeek        bool
	nextWeek        bool
	limit           int
	includeAllDay   bool
	includeDeclined bool
	sort            string
	timezone        string
	query           string
}

type eventKitStatusOptions struct {
	format  string
	json    bool
	plain   bool
	verbose bool
}

func eventKitUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical eventkit status [flags]\n  fantastical eventkit calendars [flags]\n  fantastical eventkit events [flags]\n")
	fmt.Fprint(w, "\nEXAMPLES:\n  fantastical eventkit status --json\n  fantastical eventkit calendars --json\n  fantastical eventkit events --next-week --calendar \"Work\"\n")
}

func newEventKitCalendarsFlagSet(w io.Writer) (*flag.FlagSet, *eventKitCalendarsOptions) {
	opts := &eventKitCalendarsOptions{}
	fs := flag.NewFlagSet("eventkit calendars", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&opts.format, "format", "", "Output format (plain|json|table)")
	fs.BoolVar(&opts.json, "json", false, "Print machine-readable JSON output")
	fs.BoolVar(&opts.plain, "plain", false, "Print stable plain-text output")
	fs.BoolVar(&opts.noInput, "no-input", false, "Do not prompt for Calendar access")
	fs.BoolVar(&opts.verbose, "verbose", false, "Verbose output to stderr")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical eventkit calendars [flags]\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nNOTE:\n  Requires Calendar access; macOS will prompt on first use.")
	}

	return fs, opts
}

func newEventKitEventsFlagSet(w io.Writer) (*flag.FlagSet, *eventKitEventsOptions) {
	opts := &eventKitEventsOptions{includeAllDay: true}
	fs := flag.NewFlagSet("eventkit events", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&opts.format, "format", "", "Output format (plain|json|table)")
	fs.BoolVar(&opts.json, "json", false, "Print machine-readable JSON output")
	fs.BoolVar(&opts.plain, "plain", false, "Print stable plain-text output")
	fs.BoolVar(&opts.noInput, "no-input", false, "Do not prompt for Calendar access")
	fs.BoolVar(&opts.verbose, "verbose", false, "Verbose output to stderr")
	fs.Var(&opts.calendars, "calendar", "Calendar name (repeatable)")
	fs.Var(&opts.calendarIDs, "calendar-id", "Calendar identifier (repeatable)")
	fs.StringVar(&opts.from, "from", "", "Start date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	fs.StringVar(&opts.to, "to", "", "End date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	fs.IntVar(&opts.days, "days", 0, "Days from now (shortcut for --from now --to now+days)")
	fs.BoolVar(&opts.today, "today", false, "Use today's date range")
	fs.BoolVar(&opts.tomorrow, "tomorrow", false, "Use tomorrow's date range")
	fs.BoolVar(&opts.thisWeek, "this-week", false, "Use this week's date range")
	fs.BoolVar(&opts.nextWeek, "next-week", false, "Use next week's date range")
	fs.IntVar(&opts.limit, "limit", 0, "Limit number of events returned")
	fs.BoolVar(&opts.includeAllDay, "include-all-day", true, "Include all-day events")
	fs.BoolVar(&opts.includeDeclined, "include-declined", false, "Include declined events")
	fs.StringVar(&opts.sort, "sort", "start", "Sort by start|end|title|calendar")
	fs.StringVar(&opts.timezone, "tz", "", "Timezone for output (IANA name)")
	fs.StringVar(&opts.query, "query", "", "Filter by title/location/notes (case-insensitive)")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical eventkit events [flags]\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nNOTES:\n  Requires Calendar access; macOS will prompt on first use.\n  Date shortcuts (--today/--tomorrow/--this-week/--next-week/--days) are mutually exclusive with --from/--to.")
	}

	return fs, opts
}

func newEventKitStatusFlagSet(w io.Writer) (*flag.FlagSet, *eventKitStatusOptions) {
	opts := &eventKitStatusOptions{}
	fs := flag.NewFlagSet("eventkit status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&opts.format, "format", "", "Output format (plain|json)")
	fs.BoolVar(&opts.json, "json", false, "Print machine-readable JSON output")
	fs.BoolVar(&opts.plain, "plain", false, "Print stable plain-text output")
	fs.BoolVar(&opts.verbose, "verbose", false, "Verbose output to stderr")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical eventkit status [flags]\n")
		fs.PrintDefaults()
	}

	return fs, opts
}

func cmdEventKit(args []string, out, errOut io.Writer) error {
	if len(args) < 1 {
		eventKitUsage(errOut)
		return fmt.Errorf("%w: missing eventkit subcommand", errUsage)
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "status":
		return cmdEventKitStatus(args[1:], out, errOut)
	case "calendars":
		return cmdEventKitCalendars(args[1:], out, errOut)
	case "events":
		return cmdEventKitEvents(args[1:], out, errOut)
	default:
		eventKitUsage(errOut)
		return fmt.Errorf("%w: unknown eventkit subcommand %q", errUsage, sub)
	}
}

func resolveEventKitFormat(format string, json, plain bool, allowed map[string]bool) (string, error) {
	if format != "" {
		if json || plain {
			return "", fmt.Errorf("%w: cannot combine --format with --json/--plain", errUsage)
		}
		format = strings.ToLower(strings.TrimSpace(format))
		if !allowed[format] {
			return "", fmt.Errorf("%w: invalid --format %q", errUsage, format)
		}
		return format, nil
	}
	if json && plain {
		return "", fmt.Errorf("%w: --json and --plain are mutually exclusive", errUsage)
	}
	if json {
		return "json", nil
	}
	if plain {
		return "plain", nil
	}
	if allowed["plain"] {
		return "plain", nil
	}
	return "json", nil
}

func cmdEventKitStatus(args []string, out, errOut io.Writer) error {
	fs, opts := newEventKitStatusFlagSet(errOut)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	format, err := resolveEventKitFormat(opts.format, opts.json, opts.plain, map[string]bool{
		"plain": true,
		"json":  true,
	})
	if err != nil {
		return err
	}

	helperArgs := []string{"status", "--format", format}
	return runEventKitHelper(helperArgs, out, errOut, opts.verbose)
}

func cmdEventKitCalendars(args []string, out, errOut io.Writer) error {
	fs, opts := newEventKitCalendarsFlagSet(errOut)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	format, err := resolveEventKitFormat(opts.format, opts.json, opts.plain, map[string]bool{
		"plain": true,
		"json":  true,
		"table": true,
	})
	if err != nil {
		return err
	}

	helperArgs := []string{"calendars"}
	helperArgs = append(helperArgs, "--format", format)
	if opts.noInput {
		helperArgs = append(helperArgs, "--no-input")
	}

	return runEventKitHelper(helperArgs, out, errOut, opts.verbose)
}

func cmdEventKitEvents(args []string, out, errOut io.Writer) error {
	fs, opts := newEventKitEventsFlagSet(errOut)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.Usage()
			return nil
		}
		fs.Usage()
		return fmt.Errorf("%w: %v", errUsage, err)
	}

	format, err := resolveEventKitFormat(opts.format, opts.json, opts.plain, map[string]bool{
		"plain": true,
		"json":  true,
		"table": true,
	})
	if err != nil {
		return err
	}

	helperArgs := []string{"events"}
	helperArgs = append(helperArgs, "--format", format)
	if opts.noInput {
		helperArgs = append(helperArgs, "--no-input")
	}
	for _, name := range opts.calendars {
		helperArgs = append(helperArgs, "--calendar", name)
	}
	for _, id := range opts.calendarIDs {
		helperArgs = append(helperArgs, "--calendar-id", id)
	}
	if strings.TrimSpace(opts.from) != "" {
		helperArgs = append(helperArgs, "--from", strings.TrimSpace(opts.from))
	}
	if strings.TrimSpace(opts.to) != "" {
		helperArgs = append(helperArgs, "--to", strings.TrimSpace(opts.to))
	}
	if opts.days > 0 {
		helperArgs = append(helperArgs, "--days", fmt.Sprintf("%d", opts.days))
	}
	if opts.today {
		helperArgs = append(helperArgs, "--today")
	}
	if opts.tomorrow {
		helperArgs = append(helperArgs, "--tomorrow")
	}
	if opts.thisWeek {
		helperArgs = append(helperArgs, "--this-week")
	}
	if opts.nextWeek {
		helperArgs = append(helperArgs, "--next-week")
	}
	if opts.limit > 0 {
		helperArgs = append(helperArgs, "--limit", fmt.Sprintf("%d", opts.limit))
	}
	if !opts.includeAllDay {
		helperArgs = append(helperArgs, "--no-all-day")
	}
	if opts.includeDeclined {
		helperArgs = append(helperArgs, "--include-declined")
	}
	if strings.TrimSpace(opts.sort) != "" {
		helperArgs = append(helperArgs, "--sort", strings.TrimSpace(opts.sort))
	}
	if strings.TrimSpace(opts.timezone) != "" {
		helperArgs = append(helperArgs, "--tz", strings.TrimSpace(opts.timezone))
	}
	if strings.TrimSpace(opts.query) != "" {
		helperArgs = append(helperArgs, "--query", strings.TrimSpace(opts.query))
	}

	return runEventKitHelper(helperArgs, out, errOut, opts.verbose)
}

func runEventKitHelper(args []string, out, errOut io.Writer, verbose bool) error {
	cmd, err := eventKitHelperCommand(args, errOut, verbose)
	if err != nil {
		return err
	}
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func eventKitHelperCommand(args []string, errOut io.Writer, verbose bool) (*exec.Cmd, error) {
	if override := strings.TrimSpace(os.Getenv("FANTASTICAL_EVENTKIT_HELPER")); override != "" {
		logVerbose(errOut, verbose, "eventkit helper override: %s", override)
		return exec.Command(override, args...), nil
	}

	path, err := ensureEventKitHelper(errOut, verbose)
	if err != nil {
		return nil, err
	}
	logVerbose(errOut, verbose, "eventkit helper: %s", path)
	return exec.Command(path, args...), nil
}

func ensureEventKitHelper(errOut io.Writer, verbose bool) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("eventkit cache dir: %w", err)
	}

	cacheRoot := filepath.Join(cacheDir, "fantastical")
	helperPath := filepath.Join(cacheRoot, "eventkit-helper")
	sourcePath := filepath.Join(cacheRoot, "eventkit-helper.swift")
	hashPath := filepath.Join(cacheRoot, "eventkit-helper.hash")

	hash := sha256.Sum256([]byte(eventKitHelperSource))
	hashStr := hex.EncodeToString(hash[:8])

	if _, err := os.Stat(helperPath); err == nil {
		if current, err := os.ReadFile(hashPath); err == nil && strings.TrimSpace(string(current)) == hashStr {
			return helperPath, nil
		}
	}

	if _, err := os.Stat(helperPath); err == nil {
		logVerbose(errOut, verbose, "eventkit helper hash mismatch; recompiling")
	}

	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", fmt.Errorf("eventkit cache dir: %w", err)
	}
	if err := os.WriteFile(sourcePath, []byte(eventKitHelperSource), 0o644); err != nil {
		return "", fmt.Errorf("eventkit helper source: %w", err)
	}

	compileErr := compileSwiftHelper(sourcePath, helperPath, errOut, verbose)
	if compileErr != nil {
		return "", compileErr
	}

	_ = os.WriteFile(hashPath, []byte(hashStr+"\n"), 0o644)

	return helperPath, nil
}

func compileSwiftHelper(sourcePath, outputPath string, errOut io.Writer, verbose bool) error {
	xcrunPath, err := exec.LookPath("xcrun")
	if err == nil {
		cmd := exec.Command(xcrunPath, "swiftc", "-O", "-framework", "EventKit", "-o", outputPath, sourcePath)
		cmd.Stdout = errOut
		cmd.Stderr = errOut
		logVerbose(errOut, verbose, "compiling eventkit helper with xcrun swiftc")
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	swiftcPath, err := exec.LookPath("swiftc")
	if err == nil {
		cmd := exec.Command(swiftcPath, "-O", "-framework", "EventKit", "-o", outputPath, sourcePath)
		cmd.Stdout = errOut
		cmd.Stderr = errOut
		logVerbose(errOut, verbose, "compiling eventkit helper with swiftc")
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	return errors.New("eventkit helper build failed; install Xcode Command Line Tools (xcode-select --install)")
}
