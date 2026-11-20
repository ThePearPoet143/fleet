// https://pkg.go.dev/flag#FlagSet.Visit
package setup

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
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
	a := &f.Answers
	fs.StringVar(&a.Purpose, "purpose", "", "what this machine is for: host or laptop")
	fs.StringVar(&a.Use, "use", "", "personal or shared")
	fs.StringVar(&a.GitHub.User, "github-user", "", "your GitHub username")
	fs.StringVar(&a.App.Slug, "app", "", "your agents' GitHub App, as in its URL")
	fs.Int64Var(&a.App.ID, "app-id", 0, "your agents' App ID, shown on the app's settings page")
	fs.StringVar(&a.App.Key, "key", "", "full path to this machine's app key (.pem)")
	tools := fs.String("tools", "", "AI tools, comma-separated: claude, codex")
	reach := fs.String("reach", "", "how it's reached, comma-separated: tailscale, t3, ssh")
	fs.BoolVar(&f.Yes, "yes", false, "ask nothing; use flags, current settings and what's detected")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if fs.NArg() > 0 {
		return f, fmt.Errorf("unexpected %q", fs.Arg(0))
	}
	fs.Visit(func(fl *flag.Flag) { f.Set[fl.Name] = true })
	a.Tools, a.Reach = list(*tools), list(*reach)
	errs := choices(*a)
	if f.Set["app-id"] && a.App.ID <= 0 {
		errs = append(errs, errors.New("--app-id: a positive number"))
	}
	return f, errors.Join(errs...)
}

func pick[T any](set bool, flag, current, fallback T, empty func(T) bool) T {
	if set {
		return flag
	}
	if !empty(current) {
		return current
	}
	return fallback
}

func DeriveApp(slug, oldUser, newUser string) string {
	if newUser != "" && (slug == "" || strings.EqualFold(slug, oldUser+"-agents")) {
		return newUser + "-agents"
	}
	return slug
}

func Answers(current settings.Settings, f Flags, d Detected) settings.Settings {
	str := func(s string) bool { return s == "" }
	saved := func([]string) bool { return current.Purpose == "" }
	a := f.Answers
	out := settings.Settings{
		Purpose: pick(f.Set["purpose"], a.Purpose, current.Purpose, "host", str),
		Use:     pick(f.Set["use"], a.Use, current.Use, "personal", str),
		Tools:   pick(f.Set["tools"], a.Tools, current.Tools, d.Tools, saved),
		Reach:   pick(f.Set["reach"], a.Reach, current.Reach, d.Reach, saved),
		GitHub:  settings.GitHub{User: pick(f.Set["github-user"], a.GitHub.User, current.GitHub.User, "", str)},
	}
	out.App.Key = pick(f.Set["key"], a.App.Key, current.App.Key, "", str)
	out.App.ID = pick(f.Set["app-id"], a.App.ID, current.App.ID, 0, func(id int64) bool { return id == 0 })
	slug, from := current.App.Slug, current.GitHub.User
	if f.Set["app"] {
		slug, from = a.App.Slug, ""
	}
	out.App.Slug = DeriveApp(slug, from, out.GitHub.User)
	if !f.Set["app-id"] && !strings.EqualFold(out.App.Slug, current.App.Slug) {
		out.App.ID = 0
	}
	if out.App.ID == current.App.ID && strings.EqualFold(out.App.Slug, current.App.Slug) {
		out.App.Installation = current.App.Installation
	}
	return out
}

func known(options []Option, key string) bool {
	return slices.ContainsFunc(options, func(o Option) bool { return o.Key == key })
}

func choices(s settings.Settings) []error {
	var errs []error
	if s.Purpose != "" && !known(Purposes, s.Purpose) {
		errs = append(errs, fmt.Errorf("purpose %q: use host or laptop", s.Purpose))
	}
	if s.Use != "" && !known(Uses, s.Use) {
		errs = append(errs, fmt.Errorf("use %q: use personal or shared", s.Use))
	}
	for _, t := range s.Tools {
		if !known(Tools, t) {
			errs = append(errs, fmt.Errorf("tools: %q isn't one of claude, codex", t))
		}
	}
	for _, r := range s.Reach {
		if !known(Reaches, r) {
			errs = append(errs, fmt.Errorf("reach: %q isn't one of tailscale, t3, ssh", r))
		}
	}
	return errs
}

type Lookup func(name string) (string, error)

type SignedApp func(key string, id int64) (slug, owner string, err error)

type GitHub struct {
	User         Lookup
	App          SignedApp
	Installation func(key string, id int64, user string) (int64, error)
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

func KeepID(typed string, now int64) int64 {
	if strings.TrimSpace(typed) == "" {
		return now
	}
	if id, err := AppID(typed); err == nil {
		return id
	}
	return now
}

func Check(s, current settings.Settings, gh GitHub) (settings.Settings, error) {
	var errs []error
	if !known(Purposes, s.Purpose) {
		errs = append(errs, fmt.Errorf("purpose %q: use host or laptop", s.Purpose))
		s.Purpose = current.Purpose
	}
	if !known(Uses, s.Use) {
		errs = append(errs, fmt.Errorf("use %q: use personal or shared", s.Use))
		s.Use = current.Use
	}
	if bad := choices(settings.Settings{Tools: s.Tools}); bad != nil {
		errs, s.Tools = append(errs, bad...), current.Tools
	}
	if bad := choices(settings.Settings{Reach: s.Reach}); bad != nil {
		errs, s.Reach = append(errs, bad...), current.Reach
	}
	if s.GitHub.User == "" {
		errs = append(errs, errors.New("GitHub user: needed (--github-user)"))
	} else if login, err := gh.User(s.GitHub.User); err != nil {
		errs = append(errs, err)
		if strings.EqualFold(s.App.Slug, s.GitHub.User+"-agents") {
			s.App = current.App
		}
		s.GitHub.User = current.GitHub.User
	} else {
		s.GitHub.User = login
	}
	if s.App.Slug == "" {
		errs = append(errs, errors.New("app: needed (--app)"))
	} else if s.App.ID <= 0 {
		errs = append(errs, fmt.Errorf("App ID: needed (--app-id), shown on %s", SettingsPage(s.App.Slug)))
	}
	if s.App.Key != "" && !filepath.IsAbs(s.App.Key) {
		errs, s.App.Key = append(errs, fmt.Errorf("key %q: give its full path", s.App.Key)), current.App.Key
	} else if s.App.Key != "" && s.App.ID > 0 && s.App.Slug != "" {
		if err := signIn(&s, current, gh.App); err != nil {
			errs = append(errs, err)
		}
	}
	if s.Use == "shared" {
		s.Reach = nil
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
