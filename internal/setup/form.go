// https://github.com/charmbracelet/huh/blob/v2.0.3/README.md#accessibility
package setup

import (
	"errors"
	"strconv"
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

func onGitHub(now string, user Lookup) func(string) error {
	return func(s string) error {
		name := Keep(s, now)
		if name == "" {
			return errors.New("needed")
		}
		_, err := user(name)
		return err
	}
}

func appID(now string) func(string) error {
	return func(s string) error {
		_, err := AppID(Keep(s, now))
		return err
	}
}

func Ask(a *settings.Settings, user Lookup, open func(url string)) error {
	name, app, id := "", "", ""
	oldUser := a.GitHub.User
	if err := run(huh.NewGroup(
		huh.NewSelect[string]().Title(Title("What is this machine for?", a.Purpose)).Options(options(Purposes)...).Value(&a.Purpose),
		huh.NewSelect[string]().Title(Title("Is it personal or shared?", a.Use)).Options(options(Uses)...).Value(&a.Use),
		huh.NewInput().Title(Title("Your GitHub username", a.GitHub.User)).Validate(onGitHub(a.GitHub.User, user)).Value(&name),
	)); err != nil {
		return err
	}
	a.GitHub.User = Keep(name, a.GitHub.User)
	a.App.Slug = DeriveApp(a.App.Slug, oldUser, a.GitHub.User)
	if err := run(huh.NewGroup(
		huh.NewInput().Title(Title("Your agents' GitHub App, as in its URL", a.App.Slug)).Value(&app),
		huh.NewMultiSelect[string]().Title("AI tools on this machine").Options(options(Tools)...).Value(&a.Tools),
	)); err != nil {
		return err
	}
	a.App.Slug = Keep(app, a.App.Slug)
	now := ""
	if a.App.ID > 0 {
		now = strconv.FormatInt(a.App.ID, 10)
	} else {
		open(SettingsPage(a.App.Slug))
	}
	if err := run(huh.NewGroup(
		huh.NewInput().Title(Title("Your app's App ID, shown on "+SettingsPage(a.App.Slug), now)).Validate(appID(now)).Value(&id),
	)); err != nil {
		return err
	}
	a.App.ID, _ = AppID(Keep(id, now))
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
