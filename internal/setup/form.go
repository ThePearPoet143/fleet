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
	appNow := a.App.Slug
	if appNow == "" {
		appNow = "<username>-agents"
	}
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title(Title("What is this machine for?", a.Purpose)).Options(options(Purposes)...).Value(&a.Purpose),
			huh.NewSelect[string]().Title(Title("Is it personal or shared?", a.Use)).Options(options(Uses)...).Value(&a.Use),
			huh.NewInput().Title(Title("Your GitHub username", a.GitHub.User)).Validate(neededUnless(a.GitHub.User)).Value(&user),
		),
		huh.NewGroup(
			huh.NewInput().Title(Title("Your agents' GitHub App, as in its URL", appNow)).Value(&app),
			huh.NewMultiSelect[string]().Title("AI tools on this machine").Options(options(Tools)...).Value(&a.Tools),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().Title("How is it reached?").Options(options(Reaches)...).Value(&a.Reach),
		).WithHideFunc(func() bool { return a.Use == "shared" }),
	).WithAccessible(true).Run()
	a.GitHub.User, a.App.Slug = Keep(user, a.GitHub.User), Keep(app, a.App.Slug)
	if a.App.Slug == "" && a.GitHub.User != "" {
		a.App.Slug = a.GitHub.User + "-agents"
	}
	return err
}
