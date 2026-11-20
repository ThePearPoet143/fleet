// https://pkg.go.dev/flag#FlagSet.Visit
package setup

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
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
	return f, errors.Join(choices(*a)...)
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
	if newUser != "" && (slug == "" || slug == oldUser+"-agents") {
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
	out.App.ID = current.App.ID
	slug, from := current.App.Slug, current.GitHub.User
	if f.Set["app"] {
		slug, from = a.App.Slug, ""
	}
	out.App.Slug = DeriveApp(slug, from, out.GitHub.User)
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

func Finish(s settings.Settings) (settings.Settings, error) {
	errs := choices(s)
	if s.Purpose == "" {
		errs = append(errs, errors.New("purpose: needed (--purpose)"))
	}
	if s.Use == "" {
		errs = append(errs, errors.New("use: needed (--use)"))
	}
	if s.GitHub.User == "" {
		errs = append(errs, errors.New("GitHub user: needed (--github-user)"))
	}
	if s.App.Slug == "" {
		errs = append(errs, errors.New("app: needed (--app)"))
	}
	if s.App.Key != "" && !filepath.IsAbs(s.App.Key) {
		errs = append(errs, fmt.Errorf("key %q: give its full path", s.App.Key))
	}
	if s.Use == "shared" {
		s.Reach = nil
	}
	return s, errors.Join(errs...)
}
