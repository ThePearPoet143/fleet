package token

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostsLogsGhInAsTheBot(t *testing.T) {
	want := "github.com:\n    oauth_token: ghs_example\n    git_protocol: https\n    user: someone-agents[bot]\n"
	if got := Hosts("ghs_example", "someone-agents"); got != want {
		t.Fatalf("Hosts = %q; want %q", got, want)
	}
}

func TestWriteLeavesOnlyAnOwnerOnlyHostsFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gh")
	if err := Write(dir, "ghs_example", "someone-agents"); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(filepath.Join(dir, "hosts.yml"))
	if err != nil || !strings.Contains(string(text), "oauth_token: ghs_example") {
		t.Fatalf("hosts.yml = %q, %v", text, err)
	}
	for path, want := range map[string]os.FileMode{dir: 0o700, filepath.Join(dir, "hosts.yml"): 0o600} {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: %v, %v; want %v", path, info.Mode().Perm(), err, want)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
}

func TestWriteRefusesAnEmptyTokenAndKeepsTheOldOne(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, "ghs_old", "someone-agents"); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, "", "someone-agents"); err == nil {
		t.Fatal("an empty token was written")
	}
	if text, _ := os.ReadFile(filepath.Join(dir, "hosts.yml")); !strings.Contains(string(text), "ghs_old") {
		t.Fatalf("old token lost: %q", text)
	}
}

func TestDirIsNextToSettings(t *testing.T) {
	if got := Dir("/cfg"); got != filepath.Join("/cfg", "fleet", "gh") {
		t.Fatalf("Dir = %q", got)
	}
}
