package job

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistRunsFleetTokenEvery30MinutesAndAtLogin(t *testing.T) {
	p := Plist(Job{Bin: "/Users/a&b/fleet/bin/fleet", Home: "/Users/a&b", Log: "/Users/a&b/fleet/token.log"})
	if err := xml.Unmarshal([]byte(p), new(struct{})); err != nil {
		t.Fatalf("not XML: %v\n%s", err, p)
	}
	for _, want := range []string{
		"<key>Label</key><string>dev.fleet.agent-token</string>",
		"<key>ProgramArguments</key><array><string>/Users/a&amp;b/fleet/bin/fleet</string><string>token</string></array>",
		"<key>StartInterval</key><integer>1800</integer>",
		"<key>RunAtLoad</key><true/>",
		"<key>HOME</key><string>/Users/a&amp;b</string>",
		"<key>StandardErrorPath</key><string>/Users/a&amp;b/fleet/token.log</string>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist lacks %s\n%s", want, p)
		}
	}
}

func TestInstallWritesThePlistAndReloadsTheJob(t *testing.T) {
	path := filepath.Join(t.TempDir(), "LaunchAgents", "dev.fleet.agent-token.plist")
	var ran []string
	run := func(name string, args ...string) (string, error) {
		ran = append(ran, name+" "+strings.Join(args, " "))
		if args[0] == "bootout" {
			return "Boot-out failed: 3: No such process", errors.New("exit status 3")
		}
		return "", nil
	}
	if err := Install(path, Job{Bin: "/b/fleet", Home: "/h", Log: "/l"}, 501, run); err != nil {
		t.Fatal(err)
	}
	want := []string{"launchctl bootout gui/501/dev.fleet.agent-token", "launchctl bootstrap gui/501 " + path}
	if strings.Join(ran, "\n") != strings.Join(want, "\n") {
		t.Fatalf("ran %q; want %q", ran, want)
	}
	if text, err := os.ReadFile(path); err != nil || !strings.Contains(string(text), "/b/fleet") {
		t.Fatalf("plist = %q, %v", text, err)
	}
}

func TestInstallSaysWhereToRunItWhenLaunchdRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dev.fleet.agent-token.plist")
	run := func(name string, args ...string) (string, error) {
		if args[0] == "bootstrap" {
			return "Bootstrap failed: 125: Domain does not support specified action", errors.New("exit status 125")
		}
		return "", nil
	}
	err := Install(path, Job{Bin: "/b/fleet"}, 501, run)
	if err == nil || !strings.Contains(err.Error(), "Domain does not support") || !strings.Contains(err.Error(), "at the Mac, not over SSH") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallRetriesABootstrapRightAfterBootoutAndBlamesSSHOnlyFor125(t *testing.T) {
	pause = 0
	path := filepath.Join(t.TempDir(), "dev.fleet.agent-token.plist")
	tries := 0
	busy := func(name string, args ...string) (string, error) {
		if args[0] == "bootstrap" {
			if tries++; tries == 1 {
				return "Bootstrap failed: 5: Input/output error", errors.New("exit status 5")
			}
		}
		return "", nil
	}
	if err := Install(path, Job{Bin: "/b/fleet"}, 501, busy); err != nil || tries != 2 {
		t.Fatalf("retry: %v after %d tries", err, tries)
	}
	broken := func(name string, args ...string) (string, error) {
		if args[0] == "bootstrap" {
			return "Bootstrap failed: 5: Input/output error", errors.New("exit status 5")
		}
		return "", nil
	}
	if err := Install(path, Job{Bin: "/b/fleet"}, 501, broken); err == nil || strings.Contains(err.Error(), "not over SSH") || !strings.Contains(err.Error(), "Input/output error") {
		t.Fatalf("err = %v", err)
	}
}

func TestCopyLeavesAFixedOwnerOnlyProgram(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "built")
	os.WriteFile(src, []byte("program"), 0o755)
	dst := filepath.Join(dir, "fleet", "bin", "fleet")
	if err := Copy(src, dst); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dst); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("copy: %v, %v", info, err)
	}
	if err := Copy(dst, dst); err != nil {
		t.Fatalf("copying onto itself: %v", err)
	}
	if text, _ := os.ReadFile(dst); string(text) != "program" {
		t.Fatalf("copy = %q", text)
	}
}

func TestBinIsNextToSettings(t *testing.T) {
	if Bin("/cfg") != filepath.Join("/cfg", "fleet", "bin", "fleet") || Log("/cfg") != filepath.Join("/cfg", "fleet", "token.log") {
		t.Fatal(Bin("/cfg"), Log("/cfg"))
	}
}
