// https://pkg.go.dev/os#UserConfigDir
// https://pkg.go.dev/github.com/BurntSushi/toml#MetaData.Undecoded
package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	GitHub GitHub `toml:"github"`
}

type GitHub struct {
	User string `toml:"user"`
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
