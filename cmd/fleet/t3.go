// https://github.com/pingdotgg/t3code/blob/main/docs/user/background-service.md
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/files"
	"github.com/ThePearPoet143/fleet/internal/setup"
	"github.com/ThePearPoet143/fleet/internal/token"
)

// t3Tasks are what fleet does and checks for T3 Code, the agents' supervisor; kept apart so another can replace it.
var t3Tasks = []task{
	{name: "T3 Code", check: tool("T3 Code", "t3", "curl -fsSL https://t3.codes/install.sh | sh")},
	{name: "T3's service", check: withT3(func(place) doctor.Check { return doctor.T3ServiceNow() })},
	{name: "T3 on the network", check: withT3(func(place) doctor.Check { return doctor.T3NetworkNow() })},
	{name: "T3 answers", check: withT3(func(p place) doctor.Check { return doctor.T3AnswersNow(p.home) })},
	{
		name: "T3's service.env", needs: "the token", skip: func(w *work) bool { return uninstalled(w) || w.gh.Bot == nil },
		run: func(w *work) error { return serviceEnv(w.configDir, w.s.App.Slug, w.gh.Bot, w.out) },
		check: withT3(func(p place) doctor.Check {
			return doctor.AgentEnv(doctor.T3Env(p.home), p.in(token.Dir), p.s.App.Slug)
		}),
	},
}

// withT3 checks only where T3 is installed.
func withT3(check func(place) doctor.Check) func(place) []doctor.Check {
	return func(p place) []doctor.Check {
		if doctor.Find("t3") == "" {
			return nil
		}
		return one(check(p))
	}
}

// serviceEnv makes every agent T3 starts act as the bot; T3 applies it on its next start.
func serviceEnv(configDir, slug string, bot func(string) (int64, error), out io.Writer) error {
	if _, err := os.Stat(t3Home()); err != nil {
		fmt.Fprintf(out, "T3 Code isn't set up here (no %s); skipped its service.env\n", t3Home())
		return nil
	}
	id, err := bot(slug)
	if err != nil {
		return err
	}
	path := filepath.Join(t3Home(), "service.env")
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	merged, changed := setup.MergeEnv(string(old), setup.BotLines(token.Dir(configDir), slug, id))
	if !changed {
		return os.Chmod(path, 0o600)
	}
	if err := files.Write(path, []byte(merged), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s starts agents as %s[bot]; run t3 service restart at the Mac (not over SSH) to apply it\n", path, slug)
	return nil
}
