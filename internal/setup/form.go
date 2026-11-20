// https://github.com/charmbracelet/huh/blob/v2.0.3/README.md#accessibility
package setup

import (
	"errors"
	"strings"

	"charm.land/huh/v2"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

func options(list []Option) []huh.Option[string] {
	out := make([]huh.Option[string], len(list))
	for i, o := range list {
		out[i] = huh.NewOption(o.Label, o.Key)
	}
	return out
}

func Title(question, now string) string {
	if now == "" {
		return question
	}
	return question + " (now: " + now + "; Enter keeps it)"
}

func Keep(typed, now string) string {
	if typed = strings.TrimSpace(typed); typed != "" {
		return typed
	}
	return now
}

func neededUnless(now string) func(string) error {
	return func(s string) error {
		if Keep(s, now) == "" {
			return errors.New("needed")
		}
		return nil
	}
}

func Ask(a *settings.Settings) error {
	user, app := "", ""
	oldUser := a.GitHub.User
	if err := run(huh.NewGroup(
		huh.NewSelect[string]().Title(Title("What is this machine for?", a.Purpose)).Options(options(Purposes)...).Value(&a.Purpose),
		huh.NewSelect[string]().Title(Title("Is it personal or shared?", a.Use)).Options(options(Uses)...).Value(&a.Use),
		huh.NewInput().Title(Title("Your GitHub username", a.GitHub.User)).Validate(neededUnless(a.GitHub.User)).Value(&user),
	)); err != nil {
		return err
	}
	a.GitHub.User = Keep(user, a.GitHub.User)
	a.App.Slug = DeriveApp(a.App.Slug, oldUser, a.GitHub.User)
	if err := run(huh.NewGroup(
		huh.NewInput().Title(Title("Your agents' GitHub App, as in its URL", a.App.Slug)).Value(&app),
		huh.NewMultiSelect[string]().Title("AI tools on this machine").Options(options(Tools)...).Value(&a.Tools),
	)); err != nil {
		return err
	}
	a.App.Slug = Keep(app, a.App.Slug)
	if a.Use == "shared" {
		return nil
	}
	return run(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("How is it reached?").Options(options(Reaches)...).Value(&a.Reach),
	))
}

func run(group *huh.Group) error {
	return huh.NewForm(group).WithAccessible(true).Run()
}
