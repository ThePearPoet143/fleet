// https://pkg.go.dev/bufio#Reader.ReadString
// https://github.com/charmbracelet/huh/blob/v2.0.3/internal/accessibility/accessibility.go
package setup

import (
	"bufio"
	"io"
	"strings"
)

type Lines struct {
	r       *bufio.Reader
	pending string
	EmptyAs string
}

func NewLines(r io.Reader) *Lines {
	return &Lines{r: bufio.NewReader(r)}
}

func (l *Lines) Read(p []byte) (int, error) {
	if l.pending == "" {
		line, err := l.r.ReadString('\n')
		if line == "" {
			return 0, err
		}
		if l.EmptyAs != "" && strings.TrimRight(line, "\r\n") == "" {
			line = l.EmptyAs + "\n"
		}
		l.pending = line
	}
	n := copy(p, l.pending)
	l.pending = l.pending[n:]
	return n, nil
}
