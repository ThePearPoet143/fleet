// https://pkg.go.dev/bufio#Reader.ReadString
// https://github.com/charmbracelet/huh/blob/v2.0.3/internal/accessibility/accessibility.go
package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

const longest = 1024

type Lines struct {
	r       *bufio.Reader
	pending string
	EmptyAs string
	Check   func(string) error
	Strict  bool
	Err     error
}

func NewLines(r io.Reader) *Lines {
	return &Lines{r: bufio.NewReader(r)}
}

func (l *Lines) Read(p []byte) (int, error) {
	if l.pending == "" {
		if l.Err != nil {
			return 0, io.EOF
		}
		line, err := l.r.ReadString('\n')
		if line == "" {
			return 0, err
		}
		answer := strings.TrimSpace(line)
		if answer == "" && l.EmptyAs != "" {
			answer = l.EmptyAs
		}
		if len(answer) > longest {
			l.Err = errors.New("an answer is too long")
			return 0, io.EOF
		}
		if l.Check != nil && l.Strict {
			if bad := l.Check(answer); bad != nil {
				l.Err = fmt.Errorf("answer %q: %w", answer, bad)
				return 0, io.EOF
			}
		}
		l.pending = answer + "\n"
	}
	n := copy(p, l.pending)
	l.pending = l.pending[n:]
	return n, nil
}
