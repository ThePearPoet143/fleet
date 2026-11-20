package main

import (
	"bytes"
	"testing"

	"github.com/ThePearPoet143/fleet/internal/doctor"
)

func TestRunShowsUsageForAnUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"fly"}, &out, &errOut); code != 2 || errOut.String() != usage+"\n" {
		t.Fatalf("run(fly) = %d, %q; want 2 and the usage", code, errOut.String())
	}
}

func TestReportPrintsEveryLineAndFailsWhenOneFails(t *testing.T) {
	var out bytes.Buffer
	code := report([]doctor.Check{{Name: "git", Pass: true}, {Name: "Go", Fix: "brew install go"}}, &out)
	if code != 1 || out.String() != "✓ git\n✗ Go — fix: brew install go\n" {
		t.Fatalf("report = %d, %q", code, out.String())
	}
	if report([]doctor.Check{{Name: "git", Pass: true}}, &out) != 0 {
		t.Fatal("all passing should exit 0")
	}
}
