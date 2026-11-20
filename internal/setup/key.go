// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps
// https://pkg.go.dev/os#OpenFile
package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func newKey(downloads string, since time.Time) string {
	entries, _ := os.ReadDir(downloads)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pem") {
			continue
		}
		if info, err := e.Info(); err == nil && !info.ModTime().Before(since) {
			return filepath.Join(downloads, e.Name())
		}
	}
	return ""
}

func moveKey(from, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	to := filepath.Join(dir, filepath.Base(from))
	src, err := os.Open(from)
	if err != nil {
		return "", err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("keeping the key: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(to)
		return "", err
	}
	if err := errors.Join(dst.Sync(), dst.Close(), os.Chmod(to, 0o600)); err != nil {
		os.Remove(to)
		return "", err
	}
	return to, os.Remove(from)
}

func PickUpKey(downloads, dir string, since time.Time, wait, every time.Duration) (string, error) {
	for deadline := time.Now().Add(wait); ; time.Sleep(every) {
		if from := newKey(downloads, since); from != "" {
			return moveKey(from, dir)
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no new key appeared in %s in %s", downloads, wait)
		}
	}
}
