// man launchd.plist: ProgramArguments, StartInterval, RunAtLoad, EnvironmentVariables, StandardOutPath
// man launchctl: bootstrap | bootout domain-target, gui/<uid>/[service-name]
// https://pkg.go.dev/encoding/xml#EscapeText
package job

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	Label = "dev.fleet.agent-token"
	Every = 1800
)

// Job runs a fixed copy of fleet, not the build agents work on.
type Job struct{ Bin, Home, Log string }

func Bin(configDir string) string { return filepath.Join(configDir, "fleet", "bin", "fleet") }
func Log(configDir string) string { return filepath.Join(configDir, "fleet", "token.log") }

func esc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func Plist(j Job) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + Label + `</string>
  <key>ProgramArguments</key><array><string>` + esc(j.Bin) + `</string><string>token</string></array>
  <key>EnvironmentVariables</key><dict><key>HOME</key><string>` + esc(j.Home) + `</string></dict>
  <key>StartInterval</key><integer>` + strconv.Itoa(Every) + `</integer>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>` + esc(j.Log) + `</string>
  <key>StandardErrorPath</key><string>` + esc(j.Log) + `</string>
</dict>
</plist>
`
}

type Runner func(name string, args ...string) (string, error)

// Install writes the plist and loads it in the user's login session, replacing a job of the same label.
func Install(path string, j Job, uid int, run Runner) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(Plist(j)), 0o644); err != nil {
		return err
	}
	domain := "gui/" + strconv.Itoa(uid)
	run("launchctl", "bootout", domain+"/"+Label)
	if out, err := run("launchctl", "bootstrap", domain, path); err != nil {
		return fmt.Errorf("launchctl bootstrap %s: %s (%v): start it at the Mac, not over SSH", path, strings.TrimSpace(out), err)
	}
	return nil
}

// Copy puts src at dst in one step, runnable only by its owner.
func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), "fleet-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := io.Copy(f, in); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o700); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), dst)
}
