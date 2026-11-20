package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.toml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPathIsFleetsFolderInTheConfigDir(t *testing.T) {
	got := Path("/Users/me/Library/Application Support")
	want := filepath.Join("/Users/me/Library/Application Support", "fleet", "settings.toml")
	if got != want {
		t.Fatalf("Path = %q; want %q", got, want)
	}
}

func TestLoadReadsTheGitHubUser(t *testing.T) {
	s, found, err := Load(write(t, "[github]\nuser = \"someone\"\n"))
	if err != nil || !found || s.GitHub.User != "someone" {
		t.Fatalf("Load = %+v, %v, %v; want user someone", s, found, err)
	}
}

func TestLoadSaysWhenThereAreNoSettingsYet(t *testing.T) {
	_, found, err := Load(filepath.Join(t.TempDir(), "settings.toml"))
	if err != nil || found {
		t.Fatalf("Load of a missing file = found %v, %v; want not found and no error", found, err)
	}
}

func TestLoadRefusesKeysItDoesNotKnow(t *testing.T) {
	_, _, err := Load(write(t, "[github]\nuser = \"someone\"\ntoken = \"x\"\n"))
	if err == nil || !strings.Contains(err.Error(), "github.token") {
		t.Fatalf("Load error = %v; want it to name github.token", err)
	}
}

func TestSaveWritesOnlyTheOwnersFileAndLoadsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleet", "settings.toml")
	want := Settings{Purpose: "host", Use: "personal", Tools: []string{"claude"}, GitHub: GitHub{User: "someone"},
		App: App{Slug: "someone-agents", Key: "/keys/agents.pem"}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := Load(path)
	if err != nil || !found || got.App != want.App || got.GitHub != want.GitHub || got.Purpose != "host" || len(got.Tools) != 1 {
		t.Fatalf("round trip = %+v, %v, %v; want %+v", got, found, err, want)
	}
	for p, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700 | os.ModeDir} {
		if info, err := os.Stat(p); err != nil || info.Mode() != mode {
			t.Errorf("%s mode = %v, %v; want %v", p, info.Mode(), err, mode)
		}
	}
}
