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

// skip tells that Enter passes a checked question that has no answer yet.
func skip(now string) string {
	if now == "" {
		return " (Enter skips it for now)"
	}
	return ""
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

func (f form) pick(title string, list []Option, value *string) error {
	field := huh.NewSelect[string]().Title(title).Options(options(list)...).Value(value)
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

// Ask asks each step in turn; a typed answer is checked as it's given.
func Ask(a *settings.Settings, user Lookup, open func(url string), in io.Reader, out io.Writer, strict bool) error {
	f := form{NewLines(in), out}
	f.in.Strict = strict
	before, asked := *a, map[string]bool{}
	for _, st := range Steps {
		if st.Title == "" || (st.Skip != nil && st.Skip(*a)) {
			continue
		}
		if err := f.step(st, a, user, open); err != nil {
			return err
		}
		asked[st.Flag] = true
		follow(a, before, asked)
	}
	return nil
}

func (f form) step(st Step, a *settings.Settings, user Lookup, open func(url string)) error {
	now := st.Get(*a)
	switch st.Kind {
	case One:
		return f.pick(Title(st.Title, now), st.Options, st.Value(a).(*string))
	case Many:
		return f.choose(st.Title, st.Options, st.Value(a).(*[]string))
	}
	title := st.Title
	if st.Page != nil {
		if now == "" {
			open(st.Page(*a))
		}
		title += ", shown on " + st.Page(*a)
	}
	title = Title(title, now)
	if st.Check != nil {
		title += skip(now)
	}
	typed := ""
	if err := f.text(title, st.typed(now, user), &typed); err != nil {
		return err
	}
	_ = st.Set(a, Keep(typed, now))
	return nil
}
