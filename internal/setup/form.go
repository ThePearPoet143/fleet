// https://github.com/charmbracelet/huh/blob/v2.0.3/README.md#accessibility
package setup

import (
	"io"
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

func skip(now string) string {
	if now == "" {
		return " (Enter skips it for now)"
	}
	return ""
}

func onGitHub(now string, user Lookup) func(string) error {
	return func(s string) error {
		name := Keep(s, now)
		if name == "" {
			return nil
		}
		_, err := user(name)
		return err
	}
}

func appID(now string) func(string) error {
	return func(s string) error {
		if Keep(s, now) == "" {
			return nil
		}
		_, err := AppID(Keep(s, now))
		return err
	}
}

type form struct {
	in  *Lines
	out io.Writer
}

func (f form) run(group *huh.Group) error {
	f.in.EmptyAs = ""
	return huh.NewForm(group).WithAccessible(true).WithInput(f.in).WithOutput(f.out).Run()
}

func (f form) choose(title string, list []Option, value *[]string) error {
	f.in.EmptyAs = "0"
	field := huh.NewMultiSelect[string]().Title(title + " (Enter confirms)").Options(options(list)...).Value(value)
	return huh.NewForm(huh.NewGroup(field)).WithAccessible(true).WithInput(f.in).WithOutput(f.out).Run()
}

func Ask(a *settings.Settings, user Lookup, open func(url string), in io.Reader, out io.Writer) error {
	f := form{NewLines(in), out}
	name, app := "", ""
	oldUser, oldSlug := a.GitHub.User, a.App.Slug
	if err := f.run(huh.NewGroup(
		huh.NewSelect[string]().Title(Title("What is this machine for?", a.Purpose)).Options(options(Purposes)...).Value(&a.Purpose),
		huh.NewSelect[string]().Title(Title("Is it personal or shared?", a.Use)).Options(options(Uses)...).Value(&a.Use),
		huh.NewInput().Title(Title("Your GitHub username", a.GitHub.User)+skip(a.GitHub.User)).Validate(onGitHub(a.GitHub.User, user)).Value(&name),
	)); err != nil {
		return err
	}
	a.GitHub.User = Keep(name, a.GitHub.User)
	a.App.Slug = DeriveApp(a.App.Slug, oldUser, a.GitHub.User)
	if err := f.run(huh.NewGroup(
		huh.NewInput().Title(Title("Your agents' GitHub App, as in its URL", a.App.Slug)).Value(&app),
	)); err != nil {
		return err
	}
	if err := f.choose("AI tools on this machine", Tools, &a.Tools); err != nil {
		return err
	}
	a.App.Slug = Keep(app, a.App.Slug)
	if !strings.EqualFold(a.App.Slug, oldSlug) {
		a.App.ID = 0
	}
	if err := f.askID(a, open); err != nil {
		return err
	}
	if a.Use == "shared" {
		return nil
	}
	return f.choose("How is it reached?", Reaches, &a.Reach)
}

func (f form) askID(a *settings.Settings, open func(url string)) error {
	if a.App.Slug == "" {
		return nil
	}
	id, now := "", ""
	if a.App.ID > 0 {
		now = strconv.FormatInt(a.App.ID, 10)
	} else {
		open(SettingsPage(a.App.Slug))
	}
	if err := f.run(huh.NewGroup(
		huh.NewInput().Title(Title("Your app's App ID, shown on "+SettingsPage(a.App.Slug), now) + skip(now)).Validate(appID(now)).Value(&id),
	)); err != nil {
		return err
	}
	a.App.ID = KeepID(id, a.App.ID)
	return nil
}
