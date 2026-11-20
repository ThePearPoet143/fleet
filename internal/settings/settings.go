// https://pkg.go.dev/os#UserConfigDir
// https://pkg.go.dev/github.com/BurntSushi/toml#MetaData.Undecoded
// https://pkg.go.dev/github.com/BurntSushi/toml#Encoder
// https://pkg.go.dev/os#CreateTemp
package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	Purpose string   `toml:"purpose,omitempty"`
	Use     string   `toml:"use,omitempty"`
	Tools   []string `toml:"tools"`
	Reach   []string `toml:"reach"`
	GitHub  GitHub   `toml:"github"`
	App     App      `toml:"app"`
}

type GitHub struct {
	User string `toml:"user,omitempty"`
}

type App struct {
	Slug string `toml:"slug,omitempty"`
	Key  string `toml:"key,omitempty"`
}

func Path(configDir string) string {
	return filepath.Join(configDir, "fleet", "settings.toml")
}

func Load(path string) (Settings, bool, error) {
	var s Settings
	md, err := toml.DecodeFile(path, &s)
	if errors.Is(err, fs.ErrNotExist) {
		return Settings{}, false, nil
	}
	if err != nil {
		return Settings{}, true, err
	}
	if unknown := md.Undecoded(); len(unknown) > 0 {
		names := make([]string, len(unknown))
		for i, k := range unknown {
			names[i] = k.String()
		}
		return Settings{}, true, fmt.Errorf("%s: unknown settings: %s", path, strings.Join(names, ", "))
	}
	return s, true, nil
}

func Save(path string, s Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "settings-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := toml.NewEncoder(f).Encode(s); err != nil {
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
	return os.Rename(f.Name(), path)
}
