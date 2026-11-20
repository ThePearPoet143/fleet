package setup

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

var found = Detected{Tools: []string{"claude"}, Reach: []string{"tailscale"}}

func TestParseReadsEveryQuestionsFlag(t *testing.T) {
	f, err := Parse([]string{"--purpose", "laptop", "--use", "shared", "--github-user", "someone", "--app", "bots",
		"--tools", "claude,codex", "--reach", "ssh", "--key", "/keys/a.pem", "--yes"})
	if err != nil {
		t.Fatal(err)
	}
	a := Answers(settings.Settings{}, f, found)
	if a.Purpose != "laptop" || a.Use != "shared" || a.GitHub.User != "someone" || a.App.Slug != "bots" ||
		!slices.Equal(a.Tools, []string{"claude", "codex"}) || !slices.Equal(a.Reach, []string{"ssh"}) ||
		a.App.Key != "/keys/a.pem" || !f.Yes {
		t.Fatalf("answers = %+v", a)
	}
}

func TestParseRefusesAnUnknownFlag(t *testing.T) {
	if _, err := Parse([]string{"--colour", "red"}); err == nil {
		t.Fatal("no error for --colour")
	}
}

func TestAnswersKeepCurrentSettingsOverWhatIsDetected(t *testing.T) {
	current := settings.Settings{Purpose: "laptop", Use: "personal", Tools: []string{"codex"}, GitHub: settings.GitHub{User: "someone"}, App: settings.App{Slug: "bots"}}
	f, _ := Parse([]string{"--github-user", "other"})
	a := Answers(current, f, found)
	if a.Purpose != "laptop" || !slices.Equal(a.Tools, []string{"codex"}) || a.GitHub.User != "other" || a.App.Slug != "bots" {
		t.Fatalf("answers = %+v", a)
	}
}

func TestAnswersOnAFreshMachineUseWhatIsDetected(t *testing.T) {
	f, _ := Parse([]string{"--github-user", "someone"})
	a := Answers(settings.Settings{}, f, found)
	if a.Purpose != "host" || a.Use != "personal" || a.App.Slug != "someone-agents" ||
		!slices.Equal(a.Tools, found.Tools) || !slices.Equal(a.Reach, found.Reach) {
		t.Fatalf("answers = %+v", a)
	}
}

func lookup(name string) (string, error) {
	if name == "nobody" {
		return "", errors.New(`no GitHub account "nobody"`)
	}
	return strings.ToUpper(name[:1]) + name[1:], nil
}

var good = settings.Settings{Purpose: "host", Use: "personal", GitHub: settings.GitHub{User: "someone"}, App: settings.App{ID: 42, Slug: "bots"}}

func TestCheckKeepsNoReachOnASharedMachine(t *testing.T) {
	shared := good
	shared.Use, shared.Reach = "shared", []string{"tailscale"}
	a, err := Check(shared, settings.Settings{}, lookup)
	if err != nil || a.Reach != nil {
		t.Fatalf("shared = %+v, %v; want no reach", a, err)
	}
}

func TestCheckSavesGitHubsSpellingOfTheUsername(t *testing.T) {
	if a, err := Check(good, settings.Settings{}, lookup); err != nil || a.GitHub.User != "Someone" {
		t.Fatalf("Check = %+v, %v", a, err)
	}
}

func TestCheckRefusesAnswersItCannotUse(t *testing.T) {
	cases := map[string]func(*settings.Settings){
		"purpose":     func(s *settings.Settings) { s.Purpose = "server" },
		"use":         func(s *settings.Settings) { s.Use = "family" },
		"GitHub user": func(s *settings.Settings) { s.GitHub.User = "" },
		"nobody":      func(s *settings.Settings) { s.GitHub.User = "nobody" },
		"app":         func(s *settings.Settings) { s.App.Slug = "" },
		"App ID":      func(s *settings.Settings) { s.App.ID = 0 },
		"tools":       func(s *settings.Settings) { s.Tools = []string{"vim"} },
		"reach":       func(s *settings.Settings) { s.Reach = []string{"carrier pigeon"} },
		"key":         func(s *settings.Settings) { s.App.Key = "~/a.pem" },
	}
	for want, change := range cases {
		s := good
		change(&s)
		if _, err := Check(s, settings.Settings{}, lookup); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Check with a bad %s: error %v", want, err)
		}
	}
	if _, err := Check(good, settings.Settings{}, lookup); err != nil {
		t.Errorf("good answers refused: %v", err)
	}
}

func TestCheckKeepsValidAnswersWhenOneFails(t *testing.T) {
	current := settings.Settings{GitHub: settings.GitHub{User: "Old"}}
	s := good
	s.Purpose, s.GitHub.User = "laptop", "nobody"
	a, err := Check(s, current, lookup)
	if err == nil || a.Purpose != "laptop" || a.App.ID != 42 || a.GitHub.User != "Old" {
		t.Fatalf("Check = %+v, %v; want the valid answers kept and the old user back", a, err)
	}
}

func TestAppIDIsAPositiveNumber(t *testing.T) {
	if id, err := AppID(" 42 "); err != nil || id != 42 {
		t.Errorf("AppID(42) = %d, %v", id, err)
	}
	for _, bad := range []string{"", "0", "-3", "4x"} {
		if _, err := AppID(bad); err == nil {
			t.Errorf("AppID(%q) passed", bad)
		}
	}
}

func TestTheAppsSettingsPageShowsItsID(t *testing.T) {
	if got := SettingsPage("someone-agents"); got != "https://github.com/settings/apps/someone-agents" {
		t.Errorf("SettingsPage = %q", got)
	}
}

func TestParseReadsTheAppID(t *testing.T) {
	f, err := Parse([]string{"--app-id", "42"})
	if err != nil || Answers(settings.Settings{}, f, found).App.ID != 42 {
		t.Fatalf("--app-id 42: %+v, %v", f, err)
	}
	if _, err := Parse([]string{"--app-id", "0"}); err == nil {
		t.Error("--app-id 0 passed")
	}
}

func TestARerunShowsEachCurrentAnswerAndEnterKeepsIt(t *testing.T) {
	if got := Title("Your GitHub username", "someone"); got != "Your GitHub username (now: someone; Enter keeps it)" {
		t.Errorf("title = %q", got)
	}
	if got := Title("Your GitHub username", ""); got != "Your GitHub username" {
		t.Errorf("first-run title = %q", got)
	}
	if Keep("  ", "someone") != "someone" || Keep("other", "someone") != "other" {
		t.Error("Keep should keep the current answer on Enter and take a typed one")
	}
}

func TestAnswersRememberChoosingNoToolsOrReach(t *testing.T) {
	saved := settings.Settings{Purpose: "laptop", Use: "personal", GitHub: settings.GitHub{User: "someone"}, App: settings.App{Slug: "bots"}}
	f, _ := Parse(nil)
	if a := Answers(saved, f, found); a.Tools != nil || a.Reach != nil {
		t.Fatalf("saved empty choices were refilled: %+v", a)
	}
}

func TestADerivedAppFollowsANewUsernameAndAChosenOneStays(t *testing.T) {
	derived := settings.Settings{Purpose: "host", Use: "personal", GitHub: settings.GitHub{User: "alice"}, App: settings.App{Slug: "alice-agents"}}
	f, _ := Parse([]string{"--github-user", "bob"})
	if a := Answers(derived, f, found); a.App.Slug != "bob-agents" {
		t.Errorf("derived app = %q; want bob-agents", a.App.Slug)
	}
	chosen := derived
	chosen.App.Slug = "bots"
	if a := Answers(chosen, f, found); a.App.Slug != "bots" {
		t.Errorf("chosen app = %q; want bots", a.App.Slug)
	}
	empty, _ := Parse([]string{"--github-user", "bob", "--app", ""})
	if a := Answers(chosen, empty, found); a.App.Slug != "bob-agents" {
		t.Errorf("--app \"\" = %q; want bob-agents", a.App.Slug)
	}
}

func TestParseRefusesAChoiceTheFormCannotShow(t *testing.T) {
	for _, args := range [][]string{{"--purpose", "server"}, {"--use", "family"}, {"--tools", "vim"}, {"--reach", "pigeon"}} {
		if _, err := Parse(args); err == nil {
			t.Errorf("Parse(%v) accepted it", args)
		}
	}
}

func TestADerivedAppFollowsANewUsernameWhateverItsCase(t *testing.T) {
	if got := DeriveApp("someone-agents", "Someone", "other"); got != "other-agents" {
		t.Errorf("DeriveApp = %q; want other-agents", got)
	}
}
