// https://pkg.go.dev/flag#FlagSet.Func
package setup

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

// Kind is how a step is answered.
type Kind int

const (
	One  Kind = iota // pick one of its options
	Many             // tick any of its options
	Text             // type it
)

// Step is one of setup's questions: its flag, how it's asked, and how its answer is kept and checked.
type Step struct {
	Name    string // in messages
	Flag    string
	Usage   string
	Title   string // the question; empty when only its flag answers it
	Kind    Kind
	Options []Option
	Value   func(*settings.Settings) any // the answer's place: *string, *[]string or *int64
	Default func(Detected) string        // the answer on a fresh machine
	Page    func(settings.Settings) string
	Need    func(settings.Settings) bool                     // setup isn't done without it
	Check   func(answer string, user Lookup) (string, error) // GitHub's spelling of it, or why it can't be used
	Skip    func(settings.Settings) bool                     // not asked, and not kept
	Undo    func(s *settings.Settings, current settings.Settings)
}

func always(answer string) func(Detected) string { return func(Detected) string { return answer } }

func needed(settings.Settings) bool { return true }

// Steps are setup's questions, in the order it asks them.
var Steps = []Step{
	{
		Name: "purpose", Flag: "purpose", Usage: "what this machine is for: host or laptop",
		Title: "What is this machine for?", Kind: One, Options: Purposes,
		Value:   func(s *settings.Settings) any { return &s.Purpose },
		Default: always("host"),
	},
	{
		Name: "use", Flag: "use", Usage: "personal or shared",
		Title: "Is it personal or shared?", Kind: One, Options: Uses,
		Value:   func(s *settings.Settings) any { return &s.Use },
		Default: always("personal"),
	},
	{
		Name: "GitHub user", Flag: "github-user", Usage: "your GitHub username",
		Title: "Your GitHub username", Kind: Text,
		Value: func(s *settings.Settings) any { return &s.GitHub.User },
		Need:  needed,
		Check: func(name string, user Lookup) (string, error) { return user(name) },
		Undo: func(s *settings.Settings, current settings.Settings) {
			if strings.EqualFold(s.App.Slug, s.GitHub.User+"-agents") {
				s.App = current.App
			}
			s.GitHub.User = current.GitHub.User
		},
	},
	{
		Name: "app", Flag: "app", Usage: "your agents' GitHub App, as in its URL",
		Title: "Your agents' GitHub App, as in its URL", Kind: Text,
		Value: func(s *settings.Settings) any { return &s.App.Slug },
		Need:  needed,
	},
	{
		Name: "tools", Flag: "tools", Usage: "AI tools, comma-separated: claude, codex",
		Title: "AI tools on this machine", Kind: Many, Options: Tools,
		Value:   func(s *settings.Settings) any { return &s.Tools },
		Default: func(d Detected) string { return strings.Join(d.Tools, ",") },
	},
	{
		Name: "App ID", Flag: "app-id", Usage: "your agents' App ID, shown on the app's settings page",
		Title: "Your app's App ID", Kind: Text,
		Value: func(s *settings.Settings) any { return &s.App.ID },
		Page:  func(s settings.Settings) string { return SettingsPage(s.App.Slug) },
		Need:  func(s settings.Settings) bool { return s.App.Slug != "" },
		Check: func(text string, _ Lookup) (string, error) {
			id, err := AppID(text)
			return strconv.FormatInt(id, 10), err
		},
		Skip: func(s settings.Settings) bool { return s.App.Slug == "" },
	},
	{
		Name: "reach", Flag: "reach", Usage: "how it's reached, comma-separated: tailscale, t3, ssh",
		Title: "How is it reached?", Kind: Many, Options: Reaches,
		Value:   func(s *settings.Settings) any { return &s.Reach },
		Default: func(d Detected) string { return strings.Join(d.Reach, ",") },
		Skip:    func(s settings.Settings) bool { return s.Use == "shared" },
	},
	{
		Name: "key", Flag: "key", Usage: "full path to this machine's app key (.pem)", Kind: Text,
		Value: func(s *settings.Settings) any { return &s.App.Key },
		Check: func(path string, _ Lookup) (string, error) {
			if !filepath.IsAbs(path) {
				return path, fmt.Errorf("key %q: give its full path", path)
			}
			return path, nil
		},
	},
}

func stepFor(flag string) Step {
	for _, st := range Steps {
		if st.Flag == flag {
			return st
		}
	}
	panic("no step " + flag)
}

// Get is the answer as typed: a list comma-separated, no ID as "".
func (st Step) Get(s settings.Settings) string {
	switch v := st.Value(&s).(type) {
	case *string:
		return *v
	case *[]string:
		return strings.Join(*v, ",")
	case *int64:
		if *v > 0 {
			return strconv.FormatInt(*v, 10)
		}
		return ""
	}
	panic("step " + st.Flag + ": no such answer")
}

// Set keeps a typed answer, reading an ID as flag.Int64Var does.
func (st Step) Set(s *settings.Settings, answer string) error {
	switch v := st.Value(s).(type) {
	case *string:
		*v = answer
	case *[]string:
		*v = list(answer)
	case *int64:
		if answer == "" {
			*v = 0
			return nil
		}
		id, err := strconv.ParseInt(answer, 0, 64)
		if errors.Is(err, strconv.ErrRange) {
			return errors.New("value out of range")
		} else if err != nil {
			return errors.New("parse error")
		}
		*v = id
	}
	return nil
}

func keys(options []Option, sep string) string {
	out := make([]string, len(options))
	for i, o := range options {
		out[i] = o.Key
	}
	return strings.Join(out, sep)
}

// refuse says why an answer isn't one of the step's options.
func (st Step) refuse(answer string) error {
	switch st.Kind {
	case One:
		if !known(st.Options, answer) {
			return fmt.Errorf("%s %q: use %s", st.Name, answer, keys(st.Options, " or "))
		}
	case Many:
		var errs []error
		for _, item := range list(answer) {
			if !known(st.Options, item) {
				errs = append(errs, fmt.Errorf("%s: %q isn't one of %s", st.Name, item, keys(st.Options, ", ")))
			}
		}
		return errors.Join(errs...)
	}
	return nil
}

func (st Step) check(answer string, user Lookup) (string, error) {
	if err := st.refuse(answer); err != nil {
		return answer, err
	}
	if st.Check != nil {
		return st.Check(answer, user)
	}
	return answer, nil
}

func (st Step) missing(s settings.Settings) error {
	err := fmt.Errorf("%s: needed (--%s)", st.Name, st.Flag)
	if st.Page != nil {
		err = fmt.Errorf("%w, shown on %s", err, st.Page(s))
	}
	return err
}

// typed checks an answer as typed, where Enter keeps the current one and an empty one is skipped for now.
func (st Step) typed(now string, user Lookup) func(string) error {
	if st.Check == nil {
		return nil
	}
	return func(s string) error {
		answer := Keep(s, now)
		if answer == "" {
			return nil
		}
		_, err := st.Check(answer, user)
		return err
	}
}

// follow keeps the answers that hang on others: a derived app follows a new username, unless the app was chosen,
// and another app needs its own ID, unless the ID was chosen too.
func follow(s *settings.Settings, before settings.Settings, chosen map[string]bool) {
	from := before.GitHub.User
	if chosen["app"] {
		from = ""
	}
	s.App.Slug = DeriveApp(s.App.Slug, from, s.GitHub.User)
	if !chosen["app-id"] {
		s.App.ID = 0
		if strings.EqualFold(s.App.Slug, before.App.Slug) {
			s.App.ID = before.App.ID
		}
	}
}
