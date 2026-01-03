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
	json    bool
	plain   bool
	verbose bool
	noInput bool
}

type eventKitEventsOptions struct {
	json            bool
	plain           bool
	verbose         bool
	noInput         bool
	calendars       stringSlice
	from            string
	to              string
	limit           int
	includeAllDay   bool
	includeDeclined bool
}

func eventKitUsage(w io.Writer) {
	fmt.Fprint(w, "USAGE:\n  fantastical eventkit calendars [flags]\n  fantastical eventkit events [flags]\n")
	fmt.Fprint(w, "\nEXAMPLES:\n  fantastical eventkit calendars --json\n  fantastical eventkit events --from 2026-01-03 --to 2026-01-04 --calendar \"Work\"\n")
}

func newEventKitCalendarsFlagSet(w io.Writer) (*flag.FlagSet, *eventKitCalendarsOptions) {
	opts := &eventKitCalendarsOptions{}
	fs := flag.NewFlagSet("eventkit calendars", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

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

	fs.BoolVar(&opts.json, "json", false, "Print machine-readable JSON output")
	fs.BoolVar(&opts.plain, "plain", false, "Print stable plain-text output")
	fs.BoolVar(&opts.noInput, "no-input", false, "Do not prompt for Calendar access")
	fs.BoolVar(&opts.verbose, "verbose", false, "Verbose output to stderr")
	fs.Var(&opts.calendars, "calendar", "Calendar name (repeatable)")
	fs.StringVar(&opts.from, "from", "", "Start date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	fs.StringVar(&opts.to, "to", "", "End date/time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	fs.IntVar(&opts.limit, "limit", 0, "Limit number of events returned")
	fs.BoolVar(&opts.includeAllDay, "include-all-day", true, "Include all-day events")
	fs.BoolVar(&opts.includeDeclined, "include-declined", false, "Include declined events")

	fs.Usage = func() {
		fmt.Fprint(w, "USAGE:\n  fantastical eventkit events [flags]\n")
		fs.PrintDefaults()
		fmt.Fprintln(w, "\nNOTE:\n  Requires Calendar access; macOS will prompt on first use.")
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
	case "calendars":
		return cmdEventKitCalendars(args[1:], out, errOut)
	case "events":
		return cmdEventKitEvents(args[1:], out, errOut)
	default:
		eventKitUsage(errOut)
		return fmt.Errorf("%w: unknown eventkit subcommand %q", errUsage, sub)
	}
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

	if opts.json && opts.plain {
		return fmt.Errorf("%w: --json and --plain are mutually exclusive", errUsage)
	}
	if !opts.json && !opts.plain {
		opts.plain = true
	}

	helperArgs := []string{"calendars"}
	if opts.json {
		helperArgs = append(helperArgs, "--json")
	}
	if opts.plain {
		helperArgs = append(helperArgs, "--plain")
	}
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

	if opts.json && opts.plain {
		return fmt.Errorf("%w: --json and --plain are mutually exclusive", errUsage)
	}
	if !opts.json && !opts.plain {
		opts.plain = true
	}

	helperArgs := []string{"events"}
	if opts.json {
		helperArgs = append(helperArgs, "--json")
	}
	if opts.plain {
		helperArgs = append(helperArgs, "--plain")
	}
	if opts.noInput {
		helperArgs = append(helperArgs, "--no-input")
	}
	for _, name := range opts.calendars {
		helperArgs = append(helperArgs, "--calendar", name)
	}
	if strings.TrimSpace(opts.from) != "" {
		helperArgs = append(helperArgs, "--from", strings.TrimSpace(opts.from))
	}
	if strings.TrimSpace(opts.to) != "" {
		helperArgs = append(helperArgs, "--to", strings.TrimSpace(opts.to))
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

	hash := sha256.Sum256([]byte(eventKitHelperSource))
	hashStr := hex.EncodeToString(hash[:8])
	cacheRoot := filepath.Join(cacheDir, "fantastical")
	helperPath := filepath.Join(cacheRoot, "eventkit-helper-"+hashStr)
	sourcePath := filepath.Join(cacheRoot, "eventkit-helper-"+hashStr+".swift")

	if _, err := os.Stat(helperPath); err == nil {
		return helperPath, nil
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
