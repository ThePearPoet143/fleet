package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/job"
	"github.com/ThePearPoet143/fleet/internal/rules"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/setup"
	"github.com/ThePearPoet143/fleet/internal/token"
)

// task is one thing setup does and the check the doctor makes of it; a machine check has only the check.
type task struct {
	name    string                       // in setup's messages
	answers bool                         // done while answering, before settings are saved; setupCmd runs it
	needs   string                       // a task that must have worked first
	skip    func(w *work) bool           // nothing for setup to do yet
	run     func(w *work) error          // what setup does
	check   func(p place) []doctor.Check // what the doctor checks; none where there's nothing to check
}

// work is what a task's run acts on.
type work struct {
	s         *settings.Settings
	configDir string
	gh        setup.GitHub
	wait      bool // someone is there to act and be waited for
	out       io.Writer
	errOut    io.Writer
}

// place is what a task's check looks at; configDir is "" when it's unknown.
type place struct {
	s               settings.Settings
	configDir, home string
}

// in is a path under fleet's config folder, or "" when that folder is unknown.
func (p place) in(path func(configDir string) string) string {
	if p.configDir == "" {
		return ""
	}
	return path(p.configDir)
}

// errShown is a failure the task has already printed.
var errShown = errors.New("shown")

func one(c doctor.Check) []doctor.Check { return []doctor.Check{c} }

func tool(name, program, fix string) func(place) []doctor.Check {
	return func(place) []doctor.Check { return one(doctor.Tool(name, doctor.Find(program), fix)) }
}

// machine are the checks of what fleet doesn't install.
var machine = []task{
	{name: "Homebrew", check: tool("Homebrew", "brew", "see https://brew.sh")},
	{name: "git", check: tool("git", "git", "xcode-select --install")},
	{name: "gh", check: tool("gh", "gh", "brew install gh")},
	{name: "Go", check: tool("Go", "go", "brew install go")},
	{name: "Claude Code", check: tool("Claude Code", "claude", "see https://claude.com/product/claude-code")},
	{name: "Tailscale", check: func(place) []doctor.Check { return one(doctor.TailscaleNow()) }},
	{name: "sleep", check: func(place) []doctor.Check { return one(doctor.NoSleepNow()) }},
	{name: "~/dev", check: func(p place) []doctor.Check { return one(doctor.DevFolderNow(p.home)) }},
}

var (
	keyTask = task{name: "the key", answers: true, run: func(w *work) error {
		return takeKey(w.s, filepath.Dir(settings.Path(w.configDir)), w.out)
	}}
	installTask = task{name: "the installation", answers: true, run: func(w *work) error {
		return install(w.s, w.gh, w.wait, w.out)
	}}
)

func uninstalled(w *work) bool { return w.gh.Token == nil || w.s.App.Installation <= 0 }

// shared are the tasks every machine needs, whichever supervisor and tools it has.
var shared = []task{
	keyTask,
	installTask,
	{
		name: "the token", skip: uninstalled,
		run: func(w *work) error {
			if tokenCmd(w.configDir, w.gh.Token, w.out, w.errOut) != 0 {
				return errShown
			}
			return nil
		},
		check: func(p place) []doctor.Check {
			hosts := ""
			if p.s.App.Installation > 0 {
				hosts = p.in(func(dir string) string { return filepath.Join(token.Dir(dir), "hosts.yml") })
			}
			return one(doctor.TokenNow(hosts))
		},
	},
	{
		name: "the token job", needs: "the token", skip: uninstalled,
		run: func(w *work) error {
			plist, err := agentJob(w.configDir)
			if err == nil {
				fmt.Fprintf(w.out, "the token job (%s) runs fleet token every %d minutes and at login\n", plist, job.Every/60)
			}
			return err
		},
		check: func(p place) []doctor.Check { return one(doctor.TokenJobNow(p.in(job.Bin))) },
	},
	{
		name: "ci's rules",
		run:  func(w *work) error { return agentRules(w.configDir, w.out) },
		check: func(p place) []doctor.Check {
			return one(doctor.SharedRulesNow(p.home, p.in(rules.Dir)))
		},
	},
}

// tasks are all of them, in the order setup runs them and the doctor checks them.
func tasks() []task { return slices.Concat(machine, shared, t3Tasks) }

// runTasks runs every task setup does after saving, going on past a failure; a task whose need didn't work is skipped.
func runTasks(list []task, w *work) int {
	code, worked := 0, map[string]bool{}
	for _, t := range list {
		if t.run == nil || t.answers || (t.skip != nil && t.skip(w)) || (t.needs != "" && !worked[t.needs]) {
			continue
		}
		err := t.run(w)
		worked[t.name] = err == nil
		if err != nil {
			code = 1
			if !errors.Is(err, errShown) {
				fmt.Fprintf(w.errOut, "fleet setup: %s: %v\n", t.name, err)
			}
		}
	}
	return code
}

// checkTasks makes each task's doctor check, in order.
func checkTasks(list []task, p place) []doctor.Check {
	var checks []doctor.Check
	for _, t := range list {
		if t.check != nil {
			checks = append(checks, t.check(p)...)
		}
	}
	return checks
}
