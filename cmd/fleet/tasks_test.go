package main

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/settings"
)

func TestRunTasksGoesOnPastAFailureAndSkipsWhatNeededIt(t *testing.T) {
	var ran []string
	do := func(name string, err error) func(*work) error {
		return func(*work) error { ran = append(ran, name); return err }
	}
	list := []task{
		{name: "check only", check: func(place) []doctor.Check { return nil }},
		{name: "asked", answers: true, run: do("asked", nil)},
		{name: "a", run: do("a", errors.New("no network"))},
		{name: "after a", needs: "a", run: do("after a", nil)},
		{name: "b", run: do("b", errShown)},
		{name: "skipped", skip: func(*work) bool { return true }, run: do("skipped", nil)},
		{name: "after skipped", needs: "skipped", run: do("after skipped", nil)},
		{name: "c", run: do("c", nil)},
		{name: "after c", needs: "c", run: do("after c", nil)},
	}
	var errOut bytes.Buffer
	code := runTasks(list, &work{s: &settings.Settings{}, errOut: &errOut})
	if want := []string{"a", "b", "c", "after c"}; code != 1 || !slices.Equal(ran, want) {
		t.Fatalf("runTasks = %d, ran %q; want 1, %q", code, ran, want)
	}
	if errOut.String() != "fleet setup: a: no network\n" {
		t.Fatalf("printed %q", errOut.String())
	}
	ran = nil
	if code := runTasks(list[7:], &work{s: &settings.Settings{}, errOut: &errOut}); code != 0 || len(ran) != 2 {
		t.Fatalf("all working = %d, ran %q", code, ran)
	}
}

func TestCheckTasksChecksInTheTasksOrder(t *testing.T) {
	check := func(names ...string) func(place) []doctor.Check {
		return func(place) []doctor.Check {
			var out []doctor.Check
			for _, n := range names {
				out = append(out, doctor.Check{Name: n, Pass: true})
			}
			return out
		}
	}
	list := []task{
		{name: "a", check: check("a")},
		{name: "setup only", run: func(*work) error { return nil }},
		{name: "not here", check: check()},
		{name: "b", check: check("b")},
	}
	var names []string
	for _, c := range checkTasks(list, place{}) {
		names = append(names, c.Name)
	}
	if !slices.Equal(names, []string{"a", "b"}) {
		t.Fatalf("checked %q", names)
	}
}

func TestEveryTaskDoesOrChecksSomethingAndNeedsAnEarlierOne(t *testing.T) {
	seen := map[string]bool{}
	for _, tk := range tasks() {
		if seen[tk.name] {
			t.Errorf("two tasks named %q", tk.name)
		}
		if tk.run == nil && tk.check == nil {
			t.Errorf("%q neither runs nor checks", tk.name)
		}
		if tk.answers && tk.run == nil {
			t.Errorf("%q is done while answering but has nothing to do", tk.name)
		}
		if tk.needs != "" && !seen[tk.needs] {
			t.Errorf("%q needs %q, which isn't an earlier task", tk.name, tk.needs)
		}
		seen[tk.name] = true
	}
}

func TestSetupRunsTheSharedTasksBeforeT3s(t *testing.T) {
	var names []string
	for _, tk := range tasks() {
		if tk.run != nil {
			names = append(names, tk.name)
		}
	}
	want := []string{"the key", "the installation", "the token", "the token job", "ci's rules", "T3's service.env"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("setup runs %q; want %q", names, want)
	}
}

func TestTheTokenCheckWithoutAnInstallationOrConfigFolderSaysSo(t *testing.T) {
	var checks []doctor.Check
	for _, tk := range shared {
		if tk.name == "the token" {
			checks = tk.check(place{s: settings.Settings{App: settings.App{Installation: 99}}})
		}
	}
	if len(checks) != 1 || checks[0].Pass || checks[0].Name != "agents' GitHub token fresh (no app installation in settings)" {
		t.Fatalf("token check = %+v", checks)
	}
}
