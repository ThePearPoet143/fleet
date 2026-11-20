package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/github"
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
	code := setupCmd([]string{"--github-user", "someone", "--yes"}, dir, true, setup.Detected{Tools: []string{"claude"}}, fake{}, &out, &errOut)
	s, found, err := settings.Load(settings.Path(dir))
	if code != 0 || !found || err != nil || s.App.Slug != "someone-agents" || s.Tools[0] != "claude" {
		t.Fatalf("setup = %d, %q; saved %+v, %v, %v", code, errOut.String(), s, found, err)
	}
}

func TestSetupRerunKeepsCurrentAnswers(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	setupCmd([]string{"--github-user", "someone", "--purpose", "laptop"}, dir, false, setup.Detected{}, fake{}, &out, &errOut)
	setupCmd(nil, dir, false, setup.Detected{}, fake{}, &out, &errOut)
	if s, _, _ := settings.Load(settings.Path(dir)); s.Purpose != "laptop" || s.GitHub.User != "someone" {
		t.Fatalf("rerun lost answers: %+v", s)
	}
}

func TestSetupWithoutAUserFailsAndSavesNothing(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := setupCmd(nil, dir, false, setup.Detected{}, fake{}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "--github-user") {
		t.Fatalf("setup = %d, %q", code, errOut.String())
	}
	if _, found, _ := settings.Load(settings.Path(dir)); found {
		t.Fatal("saved settings without a user")
	}
}

type fake struct{}

func (fake) User(name string) (string, error) {
	if name == "nobody" {
		return "", errors.New(`no GitHub account "nobody"`)
	}
	return name, nil
}

func (fake) App(slug, owner string) (github.App, error) {
	if slug != owner+"-agents" {
		return github.App{}, errors.New("owned by someone else")
	}
	return github.App{ID: 42, Name: slug}, nil
}

func TestSetupSavesTheAppIDGitHubGives(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	setupCmd([]string{"--github-user", "someone", "--yes"}, dir, false, setup.Detected{}, fake{}, &out, &errOut)
	if s, _, _ := settings.Load(settings.Path(dir)); s.App.ID != 42 || !strings.Contains(out.String(), "someone-agents (ID 42)") {
		t.Fatalf("saved %+v; printed %q", s, out.String())
	}
}

func TestSetupSavesNothingWhenGitHubRefuses(t *testing.T) {
	for _, args := range [][]string{{"--github-user", "nobody"}, {"--github-user", "someone", "--app", "borrowed"}} {
		dir := t.TempDir()
		var out, errOut bytes.Buffer
		if code := setupCmd(args, dir, false, setup.Detected{}, fake{}, &out, &errOut); code != 1 {
			t.Errorf("setup %v = %d", args, code)
		}
		if _, found, _ := settings.Load(settings.Path(dir)); found {
			t.Errorf("setup %v saved settings", args)
		}
	}
}
