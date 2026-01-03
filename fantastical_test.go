package main

import (
	"bytes"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestEncodeQuerySpaces(t *testing.T) {
	q := url.Values{}
	q.Set("s", "Wake up at 8am")
	enc := encodeQuery(q)
	if strings.Contains(enc, "+") {
		t.Fatalf("expected spaces encoded as %%20, got: %q", enc)
	}
	if enc != "s=Wake%20up%20at%208am" {
		t.Fatalf("unexpected encoding: %q", enc)
	}
}

func TestBuildParseURL(t *testing.T) {
	u := buildParseURL("Dinner with Sam", "Bring notes", "Work", true)
	if !strings.HasPrefix(u, fantasticalScheme+"parse?") {
		t.Fatalf("unexpected prefix: %q", u)
	}
	if strings.Contains(u, "+") {
		t.Fatalf("expected %%20 encoding, got: %q", u)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := parsed.Query()
	if q.Get("s") != "Dinner with Sam" {
		t.Fatalf("unexpected s: %q", q.Get("s"))
	}
	if q.Get("n") != "Bring notes" {
		t.Fatalf("unexpected n: %q", q.Get("n"))
	}
	if q.Get("calendarName") != "Work" {
		t.Fatalf("unexpected calendarName: %q", q.Get("calendarName"))
	}
	if q.Get("add") != "1" {
		t.Fatalf("unexpected add: %q", q.Get("add"))
	}
}

func TestParseDateArgAbsolute(t *testing.T) {
	d, err := parseDateArg("2026-01-03")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Format("2006-01-02") != "2026-01-03" {
		t.Fatalf("unexpected date: %s", d.Format("2006-01-02"))
	}
}

func TestParseDateArgRelative(t *testing.T) {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	d, err := parseDateArg("today")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Equal(midnight) {
		t.Fatalf("expected midnight %v, got %v", midnight, d)
	}
}

func TestParseDateArgInvalid(t *testing.T) {
	_, err := parseDateArg("not-a-date")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got: %v", err)
	}
}

func TestCmdParsePrintOnly(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdParse([]string{"--open=false", "--print", "Wake", "up"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errOut.String() != "" {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}

	expected := buildParseURL("Wake up", "", "", false) + "\n"
	if out.String() != expected {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestCmdParseMissingSentence(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdParse([]string{"--open=false", "--print"}, &out, &errOut); err == nil {
		t.Fatalf("expected error")
	} else if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got: %v", err)
	}
}

func TestCmdShowMiniDate(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdShow([]string{"--open=false", "--print", "mini", "2026-01-03"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := fantasticalScheme + "show/mini/2026-01-03\n"
	if out.String() != expected {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestCmdShowSet(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdShow([]string{"--open=false", "--print", "set", "My", "Calendar", "Set"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q := url.Values{}
	q.Set("name", "My Calendar Set")
	expected := fantasticalScheme + "show/set?" + encodeQuery(q) + "\n"
	if out.String() != expected {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestCmdShowTooManyArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdShow([]string{"--open=false", "--print", "mini", "2026-01-03", "extra"}, &out, &errOut); err == nil {
		t.Fatalf("expected error")
	} else if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got: %v", err)
	}
}

func TestCmdAppleScriptPrintOnly(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdAppleScript([]string{"--run=false", "--print", "Wake", "up"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "tell application \"Fantastical\"") {
		t.Fatalf("unexpected script output: %q", out.String())
	}
	if !strings.Contains(out.String(), "parse sentence theSentence") {
		t.Fatalf("unexpected script output: %q", out.String())
	}
}

func TestCmdCompletionBash(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdCompletion([]string{"bash"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "complete -F _fantastical_completions") {
		t.Fatalf("unexpected completion output: %q", out.String())
	}
}

func TestCmdCompletionUnknownShell(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdCompletion([]string{"tcsh"}, &out, &errOut); err == nil {
		t.Fatalf("expected error")
	} else if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got: %v", err)
	}
}

func TestPrintSubcommandHelpUnknown(t *testing.T) {
	if err := printSubcommandHelp("nope", &bytes.Buffer{}); err == nil {
		t.Fatalf("expected error")
	} else if !errors.Is(err, errUsage) {
		t.Fatalf("expected usage error, got: %v", err)
	}
}

func TestVersionString(t *testing.T) {
	oldVersion := version
	oldCommit := commit
	oldDate := date
	defer func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	}()

	version = "1.2.3"
	commit = "abc123"
	date = "2026-01-03"

	got := versionString()
	want := "1.2.3 abc123 2026-01-03"
	if got != want {
		t.Fatalf("unexpected version string: %q", got)
	}
}

func TestVersionStringDefault(t *testing.T) {
	oldVersion := version
	oldCommit := commit
	oldDate := date
	defer func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	}()

	version = ""
	commit = ""
	date = ""

	got := versionString()
	if got != "dev" {
		t.Fatalf("unexpected version string: %q", got)
	}
}

func TestRunVersion(t *testing.T) {
	oldVersion := version
	version = "9.9.9"
	defer func() { version = oldVersion }()

	var out, errOut bytes.Buffer
	code := run([]string{appName, "--version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "fantastical 9.9.9") {
		t.Fatalf("unexpected version output: %q", out.String())
	}
}

func TestRunHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{appName, "help", "parse"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out.String(), "fantastical parse") {
		t.Fatalf("unexpected help output: %q", out.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{appName, "wat"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Unknown command") {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}

func TestRunMissingCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{appName}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(errOut.String(), "USAGE") {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}
