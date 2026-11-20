// https://github.com/charmbracelet/huh/blob/v2.0.3/README.md#accessibility
package setup

import (
	"errors"
	"fmt"
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

func number(low, high int) func(string) error {
	return func(s string) error {
		if s == "" {
			return nil
		}
		if n, err := strconv.Atoi(s); err != nil || n < low || n > high {
			return fmt.Errorf("not a number from %d to %d", low, high)
		}
		return nil
	}
}

func (f form) ask(field huh.Field, check func(string) error, emptyAs string) (err error) {
	f.in.Check, f.in.EmptyAs = check, emptyAs
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("no valid answer was given")
		}
		if err == nil {
			err = f.in.Err
		}
	}()
	return huh.NewForm(huh.NewGroup(field)).WithAccessible(true).WithInput(f.in).WithOutput(f.out).Run()
}

func (f form) pick(title, now string, list []Option, value *string) error {
	field := huh.NewSelect[string]().Title(Title(title, now)).Options(options(list)...).Value(value)
	return f.ask(field, number(1, len(list)), "")
}

func (f form) choose(title string, list []Option, value *[]string) error {
	field := huh.NewMultiSelect[string]().Title(title + " (Enter confirms)").Options(options(list)...).Value(value)
	return f.ask(field, number(0, len(list)), "0")
}

func (f form) text(title string, check func(string) error, value *string) error {
	field := huh.NewInput().Title(title).Value(value)
	if check != nil {
		field.Validate(check)
	}
	return f.ask(field, check, "")
}

func Ask(a *settings.Settings, user Lookup, open func(url string), in io.Reader, out io.Writer, strict bool) error {
	f := form{NewLines(in), out}
	f.in.Strict = strict
	name, app := "", ""
	oldUser, oldSlug := a.GitHub.User, a.App.Slug
	if err := f.pick("What is this machine for?", a.Purpose, Purposes, &a.Purpose); err != nil {
		return err
	}
	if err := f.pick("Is it personal or shared?", a.Use, Uses, &a.Use); err != nil {
		return err
	}
	if err := f.text(Title("Your GitHub username", a.GitHub.User)+skip(a.GitHub.User), onGitHub(a.GitHub.User, user), &name); err != nil {
		return err
	}
	a.GitHub.User = Keep(name, a.GitHub.User)
	a.App.Slug = DeriveApp(a.App.Slug, oldUser, a.GitHub.User)
	if err := f.text(Title("Your agents' GitHub App, as in its URL", a.App.Slug), nil, &app); err != nil {
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
	title := Title("Your app's App ID, shown on "+SettingsPage(a.App.Slug), now) + skip(now)
	if err := f.text(title, appID(now), &id); err != nil {
		return err
	}
	a.App.ID = KeepID(id, a.App.ID)
	return nil
}
