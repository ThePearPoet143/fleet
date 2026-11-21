package setup

import (
	"testing"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

func TestEveryStepKeepsWhatItIsGiven(t *testing.T) {
	answers := map[Kind]string{One: "", Many: "", Text: "/a/b"}
	flags := map[string]bool{}
	for _, st := range Steps {
		if st.Name == "" || st.Flag == "" || st.Usage == "" || flags[st.Flag] {
			t.Errorf("step %+v needs a name, a flag of its own and its usage", st)
		}
		flags[st.Flag] = true
		answer := answers[st.Kind]
		if len(st.Options) > 0 {
			answer = st.Options[len(st.Options)-1].Key
		}
		if st.Flag == "app-id" {
			answer = "42"
		}
		var s settings.Settings
		if err := st.Set(&s, answer); err != nil || st.Get(s) != answer {
			t.Errorf("%s: set %q, got %q, %v", st.Flag, answer, st.Get(s), err)
		}
		if (st.Kind == Text) != (len(st.Options) == 0) {
			t.Errorf("%s: only a step picked from a list has options", st.Flag)
		}
	}
}

func TestTypingTheOldAppBackKeepsItsID(t *testing.T) {
	a := settings.Settings{Purpose: "host", Use: "personal", GitHub: settings.GitHub{User: "alice"}, App: settings.App{ID: 42, Slug: "alice-agents"}}
	out, opened := ask(t, &a, "\n\nbob\nalice-agents\n\n\n\n")
	if a.App.Slug != "alice-agents" || a.App.ID != 42 || opened != nil {
		t.Fatalf("app %q, ID %d, opened %v\n%s", a.App.Slug, a.App.ID, opened, out)
	}
}

func TestARenamedUserMovesADerivedAppAndAsksItsID(t *testing.T) {
	a := settings.Settings{Purpose: "host", Use: "personal", GitHub: settings.GitHub{User: "alice"}, App: settings.App{ID: 42, Slug: "alice-agents"}}
	_, opened := ask(t, &a, "\n\nbob\n\n\n7\n\n")
	if a.App.Slug != "bob-agents" || a.App.ID != 7 || len(opened) != 1 {
		t.Fatalf("app %q, ID %d, opened %v", a.App.Slug, a.App.ID, opened)
	}
}
