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
	script := "#!/bin/sh\nprintf '%s\n' \"$@\"\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	t.Setenv("FANTASTICAL_EVENTKIT_HELPER", helper)

	var out, errOut bytes.Buffer
	if err := cmdEventKit([]string{"calendars", "--plain"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "calendars") {
		t.Fatalf("expected subcommand in output: %q", output)
	}
	if !strings.Contains(output, "--format") || !strings.Contains(output, "plain") {
		t.Fatalf("expected format in output: %q", output)
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
	args := []string{"events", "--format", "json", "--calendar", "Work", "--calendar-id", "abc123", "--from", "2026-01-03", "--sort", "title", "--query", "standup", "--refresh", "--wait", "10", "--interval", "2"}
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
	if !strings.Contains(output, "--calendar-id") || !strings.Contains(output, "abc123") {
		t.Fatalf("expected calendar-id args in output: %q", output)
	}
	if !strings.Contains(output, "--from") || !strings.Contains(output, "2026-01-03") {
		t.Fatalf("expected from args in output: %q", output)
	}
	if !strings.Contains(output, "--sort") || !strings.Contains(output, "title") {
		t.Fatalf("expected sort args in output: %q", output)
	}
	if !strings.Contains(output, "--query") || !strings.Contains(output, "standup") {
		t.Fatalf("expected query args in output: %q", output)
	}
	if !strings.Contains(output, "--refresh") {
		t.Fatalf("expected refresh arg in output: %q", output)
	}
	if !strings.Contains(output, "--wait") || !strings.Contains(output, "10") {
		t.Fatalf("expected wait args in output: %q", output)
	}
	if !strings.Contains(output, "--interval") || !strings.Contains(output, "2") {
		t.Fatalf("expected interval args in output: %q", output)
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

func TestCmdEventKitStatusArgs(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "helper.sh")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\"\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	t.Setenv("FANTASTICAL_EVENTKIT_HELPER", helper)

	var out, errOut bytes.Buffer
	if err := cmdEventKit([]string{"status", "--format", "json"}, &out, &errOut); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "status") || !strings.Contains(output, "--format") || !strings.Contains(output, "json") {
		t.Fatalf("expected status args in output: %q", output)
	}
}
