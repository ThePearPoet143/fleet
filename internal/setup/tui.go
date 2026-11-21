// https://github.com/charmbracelet/huh/blob/v2.0.3/README.md#dynamic-forms
// https://github.com/charmbracelet/huh/blob/v2.0.3/form.go (prevGroupMsg, WithWidth)
// https://github.com/charmbracelet/bubbles/blob/v2.0.0/textinput/textinput.go (placeholderView)
package setup

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"

	"github.com/ThePearPoet143/fleet/internal/settings"
)

// ErrStopped is a form left with ctrl+c, its only quit key.
var ErrStopped = errors.New("stopped; nothing saved")

// typing is the form's state: the picked answers, and what's typed in each text step.
type typing struct {
	A     *settings.Settings
	Typed map[string]*string
	mu    *sync.Mutex
}

// guarded holds an answer behind the form's lock: huh runs titles, hints and placeholders in goroutines.
type guarded[T any] struct {
	mu    *sync.Mutex
	value *T
}

func (g guarded[T]) Get() T {
	g.mu.Lock()
	defer g.mu.Unlock()
	return *g.value
}

func (g guarded[T]) Set(value T) {
	g.mu.Lock()
	defer g.mu.Unlock()
	*g.value = value
}

// answers are the settings as they stand, without what's typed in one step; Enter keeps what's shown.
func (t typing) answers(before settings.Settings, except string) settings.Settings {
	if t.mu != nil {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	s, chosen := *t.A, map[string]bool{}
	for flag, typed := range t.Typed {
		if answer := strings.TrimSpace(*typed); answer != "" && flag != except {
			_ = stepFor(flag).Set(&s, answer)
			chosen[flag] = true
		}
	}
	follow(&s, before, chosen)
	return s
}

// hidden is a step the form leaves out; it shows the App ID step before there's an app, so the count holds.
func hidden(st Step, s settings.Settings) bool { return st.Drop && st.Skip(s) }

// shown are the steps the form shows for these answers.
func shown(s settings.Settings) []Step {
	var out []Step
	for _, st := range Steps {
		if st.Title != "" && !hidden(st, s) {
			out = append(out, st)
		}
	}
	return out
}

// Numbered is a step's title with where it stands, such as "Step 3 of 7 · Your GitHub username".
func Numbered(st Step, s settings.Settings) string {
	list := shown(s)
	for i, at := range list {
		if at.Flag == st.Flag {
			return fmt.Sprintf("Step %d of %d · %s", i+1, len(list), st.Title)
		}
	}
	return st.Title
}

// Hint says where a text answer is shown and what Enter does.
func Hint(st Step, s settings.Settings) string {
	var parts []string
	if st.Page != nil && st.Page(s) != "" {
		parts = append(parts, "Shown on "+st.Page(s))
	}
	switch now := st.Get(s); {
	case now != "":
		parts = append(parts, "Enter keeps "+now)
	case st.Check != nil:
		parts = append(parts, "Enter skips it for now")
	}
	return strings.Join(parts, "; ")
}

// remember asks GitHub once per account found: huh checks an answer on Enter and again as it moves on.
// A refusal isn't kept, since it may pass on a retry (GitHub's rate limit, the network).
func remember(user Lookup) Lookup {
	found := map[string]string{}
	return func(name string) (string, error) {
		if login, ok := found[name]; ok {
			return login, nil
		}
		login, err := user(name)
		if err == nil {
			found[name] = login
		}
		return login, err
	}
}

// Size is the form's size. huh measures its groups before the titles are filled in, so the height is fixed,
// and under 10 rows the form drops its help line to keep the title; the width is huh's own unless the
// terminal gives none, as huh sizes a text input from it and 0 panics.
func Size(out io.Writer) (width, height int) {
	if f, ok := out.(interface{ Fd() uintptr }); ok {
		if w, h, err := term.GetSize(f.Fd()); err == nil && w > 0 && h > 0 {
			return 0, min(10, h)
		}
	}
	return 80, 10
}

// text is a text step. Going back, its error doesn't count: huh blurs the field, then refuses to leave a group
// with an error (form.go, prevGroupMsg) and never focuses it again, which stops the form. Enter checks it again.
type text struct {
	*huh.Input
	focus func()
	back  bool
}

func (f *text) Focus() tea.Cmd {
	f.back = false
	if f.focus != nil {
		f.focus()
	}
	return f.Input.Focus()
}

func (f *text) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		f.back = key.Matches(k, huh.NewDefaultKeyMap().Input.Prev)
	}
	m, cmd := f.Input.Update(msg)
	f.Input = m.(*huh.Input)
	return f, cmd
}

func (f *text) Error() error {
	if f.back {
		return nil
	}
	return f.Input.Error()
}

// Form asks every step in one form, a group per step; shift+tab goes back.
func Form(a *settings.Settings, user Lookup, open func(url string), in io.Reader, out io.Writer) error {
	before := *a
	t := &typing{A: a, Typed: map[string]*string{}, mu: &sync.Mutex{}}
	user = remember(user)
	opened := ""
	var groups []*huh.Group
	for _, st := range Steps {
		if st.Title == "" {
			continue
		}
		title := func() string { return Numbered(st, t.answers(before, "")) }
		var field huh.Field
		switch st.Kind {
		case One:
			field = huh.NewSelect[string]().TitleFunc(title, t).Options(options(st.Options)...).
				Accessor(guarded[string]{t.mu, st.Value(a).(*string)})
		case Many:
			// Without a height, huh v2.0.3 takes the title and description off the options' own height
			// (field_multiselect.go:504-514), and two options show none.
			field = huh.NewMultiSelect[string]().TitleFunc(title, t).Description("x or space ticks, Enter confirms").
				Options(options(st.Options)...).Accessor(guarded[[]string]{t.mu, st.Value(a).(*[]string)}).
				Height(2 + len(st.Options))
		case Text:
			typed := new(string)
			t.Typed[st.Flag] = typed
			now := func() settings.Settings { return t.answers(before, st.Flag) }
			input := huh.NewInput().TitleFunc(title, t).Accessor(guarded[string]{t.mu, typed}).
				PlaceholderFunc(func() string { return st.Get(now()) }, t).
				DescriptionFunc(func() string { return Hint(st, now()) }, t).
				Validate(func(s string) error {
					if st.Skip != nil && st.Skip(now()) && strings.TrimSpace(s) != "" {
						return errors.New("there's no app yet: shift+tab back to give it")
					}
					if check := st.typed(st.Get(now()), user); check != nil {
						return check(s)
					}
					return nil
				})
			f := &text{Input: input}
			if st.Page != nil {
				f.focus = func() {
					if s := now(); st.Get(s) == "" && st.Page(s) != "" && st.Page(s) != opened {
						opened = st.Page(s)
						open(opened)
					}
				}
			}
			field = f
		}
		group := huh.NewGroup(field)
		if st.Drop {
			group.WithHideFunc(func() bool { return hidden(st, t.answers(before, "")) })
		}
		groups = append(groups, group)
	}
	width, height := Size(out)
	err := huh.NewForm(groups...).WithInput(in).WithOutput(out).WithWidth(width).WithHeight(height).
		WithShowHelp(height >= 10).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		*a = before
		return ErrStopped
	}
	if err != nil {
		*a = before
		return err
	}
	*a = t.answers(before, "")
	return nil
}
