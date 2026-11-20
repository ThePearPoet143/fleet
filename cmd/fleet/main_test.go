package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/setup"
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

func TestSetupWithFlagsAsksNothingAndSaves(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--yes"}, dir, strings.NewReader(""), setup.Detected{Tools: []string{"claude"}}, fake, &out, &errOut)
	s, found, err := settings.Load(settings.Path(dir))
	if code != 0 || !found || err != nil || s.App.Slug != "someone-agents" || s.App.ID != 42 || s.Tools[0] != "claude" {
		t.Fatalf("setup = %d, %q; saved %+v, %v, %v", code, errOut.String(), s, found, err)
	}
}

func TestSetupRerunKeepsCurrentAnswers(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--purpose", "laptop"}, dir, strings.NewReader(""), setup.Detected{}, fake, &out, &errOut)
	if code := setupCmd(nil, dir, strings.NewReader(""), setup.Detected{}, fake, &out, &errOut); code != 0 {
		t.Fatalf("rerun = %d, %q", code, errOut.String())
	}
	if s, _, _ := settings.Load(settings.Path(dir)); s.Purpose != "laptop" || s.GitHub.User != "someone" || s.App.ID != 42 {
		t.Fatalf("rerun lost answers: %+v", s)
	}
}

func TestSetupKeepsValidAnswersWhenOneFails(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "nobody", "--app", "bots", "--app-id", "42", "--purpose", "laptop"}, dir, strings.NewReader(""), setup.Detected{}, fake, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), `no GitHub account "nobody"`) {
		t.Fatalf("setup = %d, %q", code, errOut.String())
	}
	if s, found, _ := settings.Load(settings.Path(dir)); !found || s.Purpose != "laptop" || s.App.ID != 42 || s.GitHub.User != "" {
		t.Fatalf("saved %+v, %v; want the valid answers and no user", s, found)
	}
}

func fake(name string) (string, error) {
	if name == "nobody" {
		return "", errors.New(`no GitHub account "nobody"`)
	}
	return name, nil
}

func TestPipedAnswersDriveTheForm(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd(nil, dir, strings.NewReader("1\n1\nsomeone\n\n\n42\n\n"), setup.Detected{Tools: []string{"claude"}}, fake, &out, &errOut)
	s, _, _ := settings.Load(settings.Path(dir))
	if code != 0 || s.GitHub.User != "someone" || s.App.ID != 42 || s.Tools[0] != "claude" {
		t.Fatalf("setup = %d, %q; saved %+v", code, errOut.String(), s)
	}
}
