// https://pkg.go.dev/flag#FlagSet.Visit
package setup

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

type Option struct{ Key, Label string }

var (
	Purposes = []Option{{"host", "agents run here"}, {"laptop", "I work here"}}
	Uses     = []Option{{"personal", "personal"}, {"shared", "shared, such as a work PC"}}
	Tools    = []Option{{"claude", "Claude Code"}, {"codex", "Codex"}}
	Reaches  = []Option{{"tailscale", "Tailscale"}, {"t3", "T3 Code on the network"}, {"ssh", "SSH"}}
)

type Flags struct {
	Answers settings.Settings
	Set     map[string]bool
	Yes     bool
}

type Detected struct{ Tools, Reach []string }

func list(text string) []string {
	var out []string
	for _, item := range strings.Split(text, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func Parse(args []string) (Flags, error) {
	f := Flags{Set: map[string]bool{}}
	fs := flag.NewFlagSet("fleet setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	for _, st := range Steps {
		fs.Func(st.Flag, st.Usage, func(answer string) error { return st.Set(&f.Answers, answer) })
	}
	fs.BoolVar(&f.Yes, "yes", false, "ask nothing; use flags, current settings and what's detected")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if fs.NArg() > 0 {
		return f, fmt.Errorf("unexpected %q", fs.Arg(0))
	}
	fs.Visit(func(fl *flag.Flag) { f.Set[fl.Name] = true })
	var errs []error
	for _, st := range Steps {
		if answer := st.Get(f.Answers); f.Set[st.Flag] && (answer != "" || st.Kind == Many) {
			errs = append(errs, st.refuse(answer))
		}
	}
	if f.Set["app-id"] && f.Answers.App.ID <= 0 {
		errs = append(errs, errors.New("--app-id: a positive number"))
	}
	return f, errors.Join(errs...)
}

func DeriveApp(slug, oldUser, newUser string) string {
	if newUser != "" && (slug == "" || strings.EqualFold(slug, oldUser+"-agents")) {
		return newUser + "-agents"
	}
	return slug
}

// Answers takes each answer from its flag, else the current settings, else what's detected.
func Answers(current settings.Settings, f Flags, d Detected) settings.Settings {
	var out settings.Settings
	for _, st := range Steps {
		answer := st.Get(current)
		saved := answer != ""
		if st.Kind == Many {
			saved = current.Purpose != ""
		}
		switch {
		case f.Set[st.Flag]:
			answer = st.Get(f.Answers)
		case !saved && st.Default != nil:
			answer = st.Default(d)
		}
		_ = st.Set(&out, answer)
	}
	follow(&out, current, f.Set)
	if out.App.ID == current.App.ID && strings.EqualFold(out.App.Slug, current.App.Slug) {
		out.App.Installation = current.App.Installation
	}
	return out
}

func known(options []Option, key string) bool {
	return slices.ContainsFunc(options, func(o Option) bool { return o.Key == key })
}

type Lookup func(name string) (string, error)

type SignedApp func(key string, id int64) (slug, owner string, err error)

type GitHub struct {
	User         Lookup
	App          SignedApp
	Installation func(key string, id int64, user string) (int64, error)
	Token        func(key string, id, installation int64) (string, error)
	Bot          func(slug string) (int64, error)
}

func SettingsPage(slug string) string {
	return "https://github.com/settings/apps/" + url.PathEscape(slug)
}

func AppID(text string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("the App ID is a positive number")
	}
	return id, nil
}

// Check puts back the current answer wherever one can't be used, and says why.
func Check(s, current settings.Settings, gh GitHub) (settings.Settings, error) {
	var errs []error
	failed := map[string]bool{}
	for _, st := range Steps {
		answer := st.Get(s)
		if answer == "" && st.Kind != One {
			if st.Need != nil && st.Need(s) {
				errs = append(errs, st.missing(s))
			}
			continue
		}
		fixed, err := st.check(answer, gh.User)
		if err == nil {
			_ = st.Set(&s, fixed)
			continue
		}
		errs, failed[st.Flag] = append(errs, err), true
		if st.Undo != nil {
			st.Undo(&s, current)
		} else {
			_ = st.Set(&s, st.Get(current))
		}
	}
	if !failed["key"] && s.App.Key != "" && s.App.ID > 0 && s.App.Slug != "" {
		if err := signIn(&s, current, gh.App); err != nil {
			errs = append(errs, err)
		}
	}
	for _, st := range Steps {
		if st.Skip != nil && st.Skip(s) {
			_ = st.Set(&s, "")
		}
	}
	return s, errors.Join(errs...)
}

func signIn(s *settings.Settings, current settings.Settings, app SignedApp) error {
	slug, owner, err := app(s.App.Key, s.App.ID)
	switch {
	case err != nil:
		err = fmt.Errorf("signing in as the app with %s: %w", s.App.Key, err)
	case !strings.EqualFold(slug, s.App.Slug):
		err = fmt.Errorf("App ID %d is %q, not %q: check the App ID or the app's name", s.App.ID, slug, s.App.Slug)
	case !strings.EqualFold(owner, s.GitHub.User):
		err = fmt.Errorf("the app %q is owned by %q, not %q: fleet works with an app owned by your own account", slug, owner, s.GitHub.User)
	default:
		s.App.Slug = slug
		return nil
	}
	s.App = current.App
	return err
}
