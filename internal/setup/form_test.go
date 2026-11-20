package setup

import (
	"bufio"
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

func ask(t *testing.T, a *settings.Settings, input string) (string, []string) {
	t.Helper()
	var out bytes.Buffer
	var opened []string
	if err := Ask(a, lookup, func(url string) { opened = append(opened, url) }, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	return out.String(), opened
}

func TestLinesHandsOutOneLineAtATime(t *testing.T) {
	l := NewLines(strings.NewReader("1\n\nabc\n"))
	var got []string
	for range 3 {
		s := bufio.NewScanner(l)
		s.Scan()
		got = append(got, s.Text())
		l.EmptyAs = "0"
	}
	if !slices.Equal(got, []string{"1", "0", "abc"}) {
		t.Fatalf("got %q", got)
	}
}

func TestPipedAnswersFillEveryQuestion(t *testing.T) {
	a := settings.Settings{Purpose: "host", Use: "personal", Tools: []string{"claude"}}
	out, opened := ask(t, &a, "2\n1\noctocat\n\n2\n\n42\n\n")
	if a.Purpose != "laptop" || a.GitHub.User != "octocat" || a.App.Slug != "octocat-agents" || a.App.ID != 42 ||
		!slices.Equal(a.Tools, []string{"claude", "codex"}) {
		t.Fatalf("answers = %+v\n%s", a, out)
	}
	if !slices.Equal(opened, []string{"https://github.com/settings/apps/octocat-agents"}) {
		t.Errorf("opened %v", opened)
	}
}

func TestARerunShowsTheSavedAppIDAndEnterKeepsIt(t *testing.T) {
	a := settings.Settings{Purpose: "host", Use: "personal", GitHub: settings.GitHub{User: "octocat"}, App: settings.App{ID: 42, Slug: "octocat-agents"}}
	out, opened := ask(t, &a, "\n\n\n\n\n\n\n")
	if !strings.Contains(out, "now: 42; Enter keeps it") || a.App.ID != 42 || opened != nil {
		t.Fatalf("ID %d, opened %v\n%s", a.App.ID, opened, out)
	}
}

func TestEnterConfirmsAMultiChoiceList(t *testing.T) {
	a := settings.Settings{Purpose: "host", Use: "personal", Tools: []string{"codex"}, Reach: []string{"ssh"}, GitHub: settings.GitHub{User: "octocat"}, App: settings.App{ID: 42, Slug: "octocat-agents"}}
	out, _ := ask(t, &a, "\n\n\n\n\n\n\n")
	if strings.Contains(out, "Invalid") || !slices.Equal(a.Tools, []string{"codex"}) || !slices.Equal(a.Reach, []string{"ssh"}) {
		t.Fatalf("tools %v, reach %v\n%s", a.Tools, a.Reach, out)
	}
}

func TestTheEndOfInputKeepsEveryAnswer(t *testing.T) {
	saved := settings.Settings{Purpose: "laptop", Use: "personal", Tools: []string{"codex"}, GitHub: settings.GitHub{User: "octocat"}, App: settings.App{ID: 42, Slug: "octocat-agents"}}
	a := saved
	ask(t, &a, "")
	if a.Purpose != "laptop" || a.GitHub.User != "octocat" || a.App.ID != 42 || !slices.Equal(a.Tools, []string{"codex"}) {
		t.Fatalf("answers = %+v; want %+v", a, saved)
	}
}
