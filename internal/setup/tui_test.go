package setup

import (
	"bytes"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

func TestEachStepShowsWhereItStands(t *testing.T) {
	s := settings.Settings{Use: "personal", App: settings.App{Slug: "a-agents"}}
	if got := Numbered(stepFor("github-user"), s); got != "Step 3 of 7 · Your GitHub username" {
		t.Errorf("personal: %q", got)
	}
	s.Use = "shared"
	if got := Numbered(stepFor("app-id"), s); got != "Step 6 of 6 · Your app's App ID" {
		t.Errorf("shared: %q", got)
	}
	s.App.Slug, s.Use = "", "personal"
	if got := Numbered(stepFor("tools"), s); got != "Step 5 of 7 · AI tools on this machine" {
		t.Errorf("no app yet: %q", got)
	}
}

func TestTheFormFollowsWhatIsTyped(t *testing.T) {
	before := settings.Settings{Use: "personal", GitHub: settings.GitHub{User: "alice"}, App: settings.App{ID: 42, Slug: "alice-agents"}}
	a := before
	user, app := "bob", ""
	ty := typing{A: &a, Typed: map[string]*string{"github-user": &user, "app": &app, "app-id": new(string)}}
	if s := ty.answers(before, ""); s.GitHub.User != "bob" || s.App.Slug != "bob-agents" || s.App.ID != 0 {
		t.Errorf("renamed: %+v", s)
	}
	if s := ty.answers(before, "github-user"); s.GitHub.User != "alice" || s.App.ID != 42 {
		t.Errorf("without the typed name: %+v", s)
	}
	app = " alice-agents "
	if s := ty.answers(before, ""); s.App.Slug != "alice-agents" || s.App.ID != 42 {
		t.Errorf("the old app typed back: %+v", s)
	}
}

func TestATextStepSaysWhatEnterDoes(t *testing.T) {
	s := settings.Settings{GitHub: settings.GitHub{User: "octocat"}, App: settings.App{Slug: "octocat-agents"}}
	cases := map[string]string{
		"github-user": "Enter keeps octocat",
		"app-id":      "Shown on https://github.com/settings/apps/octocat-agents; Enter skips it for now",
	}
	for flag, want := range cases {
		if got := Hint(stepFor(flag), s); got != want {
			t.Errorf("%s: %q; want %q", flag, got, want)
		}
	}
	if got := Hint(stepFor("app"), settings.Settings{}); got != "" {
		t.Errorf("an empty app: %q", got)
	}
	if got := Hint(stepFor("app-id"), settings.Settings{}); got != "Enter skips it for now" {
		t.Errorf("an App ID before there's an app: %q", got)
	}
}

func TestTheFormAsksGitHubOncePerName(t *testing.T) {
	calls := 0
	user := remember(func(name string) (string, error) { calls++; return lookup(name) })
	for range 3 {
		user("octocat")
		user("nobody")
	}
	if calls != 4 {
		t.Errorf("%d lookups; want 4: octocat once, nobody each time", calls)
	}
}

func TestAFormWithoutATerminalSizeStillHasAWidth(t *testing.T) {
	if w, h := Size(&bytes.Buffer{}); w != 80 || h != 10 {
		t.Errorf("size %d×%d; want 80×10", w, h)
	}
}

func TestAWrongAnswerDoesNotStopGoingBack(t *testing.T) {
	f := &text{Input: huh.NewInput().Validate(func(string) error { return errors.New("no") })}
	f.WithKeyMap(huh.NewDefaultKeyMap())
	f.Focus()
	f.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if f.Error() == nil {
		t.Fatal("Enter passed a wrong answer")
	}
	f.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	f.Blur()
	if f.Error() != nil {
		t.Errorf("shift+tab was held up: %v", f.Error())
	}
}
