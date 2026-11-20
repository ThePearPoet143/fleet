// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps
// https://pkg.go.dev/io/fs#FileMode.IsRegular
// https://pkg.go.dev/os#OpenFile
package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ThePearPoet143/fleet/internal/github"
)

const largestKey = 64 << 10

func readKey(path string) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, largestKey))
	return b, err == nil && github.CheckKey(b) == nil
}

func newKey(downloads string, since time.Time) (string, []byte) {
	entries, _ := os.ReadDir(downloads)
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".pem") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().Before(since) {
			continue
		}
		if b, ok := readKey(filepath.Join(downloads, e.Name())); ok {
			return filepath.Join(downloads, e.Name()), b
		}
	}
	return "", nil
}

func numbered(name string, n int) string {
	if n == 1 {
		return name
	}
	i := strings.Index(name, ".")
	if i < 0 {
		i = len(name)
	}
	return name[:i] + "-" + strconv.Itoa(n) + name[i:]
}

func keep(key []byte, name, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		to := filepath.Join(dir, numbered(name, n))
		f, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("keeping the key: %w", err)
		}
		_, err = f.Write(key)
		if err = errors.Join(err, f.Sync(), f.Close(), os.Chmod(to, 0o600)); err != nil {
			os.Remove(to)
			return "", err
		}
		return to, nil
	}
}

func PickUpKey(downloads, dir string, since time.Time, wait, every time.Duration) (string, error) {
	for deadline := time.Now().Add(wait); ; time.Sleep(every) {
		if from, key := newKey(downloads, since); from != "" {
			to, err := keep(key, filepath.Base(from), dir)
			if err != nil {
				return "", err
			}
			if err := os.Remove(from); err != nil {
				return to, fmt.Errorf("the key is kept, but its download wasn't removed: %w", err)
			}
			return to, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no new key appeared in %s in %s", downloads, wait)
		}
	}
}
