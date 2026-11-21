// https://cli.github.com/manual/gh_help_environment
// https://pkg.go.dev/os#CreateTemp
// https://pkg.go.dev/os#Rename
package token

import (
	"errors"
	"os"
	"path/filepath"
)

// Dir is the gh config folder agents use (GH_CONFIG_DIR), next to fleet's settings.
func Dir(configDir string) string {
	return filepath.Join(configDir, "fleet", "gh")
}

func Hosts(token, slug string) string {
	return "github.com:\n    oauth_token: " + token + "\n    git_protocol: https\n    user: " + slug + "[bot]\n"
}

// Write replaces dir/hosts.yml in one step, readable only by its owner; an empty token leaves the old one.
func Write(dir, token, slug string) error {
	if token == "" {
		return errors.New("no token; the old login is kept")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "hosts-*.yml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(Hosts(token, slug)); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "hosts.yml"))
}
