//go:build darwin
// +build darwin

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdEventKitCalendarsHelperOverride(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "helper.sh")
	script := "#!/bin/sh\necho calendars-ok\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	t.Setenv("FANTASTICAL_EVENTKIT_HELPER", helper)

	var out, errOut bytes.Buffer
	if err := cmdEventKit([]string{"calendars", "--plain"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.String() != "calendars-ok\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestCmdEventKitEventsArgs(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "helper.sh")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\"\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	t.Setenv("FANTASTICAL_EVENTKIT_HELPER", helper)

	var out, errOut bytes.Buffer
	args := []string{"events", "--json", "--calendar", "Work", "--from", "2026-01-03"}
	if err := cmdEventKit(args, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "events") {
		t.Fatalf("expected subcommand in output: %q", output)
	}
	if !strings.Contains(output, "--calendar") || !strings.Contains(output, "Work") {
		t.Fatalf("expected calendar args in output: %q", output)
	}
	if !strings.Contains(output, "--from") || !strings.Contains(output, "2026-01-03") {
		t.Fatalf("expected from args in output: %q", output)
	}
}

func TestCmdEventKitMissingSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := cmdEventKit([]string{}, &out, &errOut); err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(errOut.String(), "USAGE") {
		t.Fatalf("expected usage in stderr: %q", errOut.String())
	}
}
