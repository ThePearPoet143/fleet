// https://pkg.go.dev/os#UserConfigDir
// https://pkg.go.dev/os#FileMode
// https://ss64.com/mac/open.html
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/github"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/setup"
	"github.com/ThePearPoet143/fleet/internal/system"
)

const usage = "usage: fleet doctor | fleet setup [--purpose host|laptop] [--use personal|shared] [--github-user name]\n" +
	"  [--app slug] [--app-id n] [--tools claude,codex] [--reach tailscale,t3,ssh] [--key /full/path.pem] [--yes]"

var (
	open         = func(url string) {}
	piped        = true
	keyWait      = 5 * time.Minute
	installWait  = 5 * time.Minute
	installEvery = 3 * time.Second
	downloads    = func() string {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Downloads")
	}
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || (args[0] != "doctor" && args[0] != "setup") || (args[0] == "doctor" && len(args) > 1) {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	if _, err := system.Detect(runtime.GOOS); err != nil {
		fmt.Fprintf(errOut, "fleet %s: %v\n", args[0], err)
		return 2
	}
	if args[0] == "setup" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(errOut, "fleet setup:", err)
			return 2
		}
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			open = func(url string) { _ = exec.Command("open", url).Start() }
			piped = false
		}
		if code := setupCmd(args[1:], dir, os.Stdin, setup.Detect(), setup.GitHub{User: github.New().User, App: signIn(github.New()), Installation: installation(github.New())}, out, errOut); code != 0 {
			return code
		}
	}
	return doctorCmd(out, errOut)
}

func setupCmd(args []string, configDir string, in io.Reader, found setup.Detected, gh setup.GitHub, out, errOut io.Writer) int {
	f, err := setup.Parse(args)
	if err != nil {
		fmt.Fprintf(errOut, "fleet setup: %v\n%s\n", err, usage)
		return 2
	}
	path := settings.Path(configDir)
	current, _, err := settings.Load(path)
	if err != nil {
		fmt.Fprintln(errOut, "fleet setup:", err)
		return 2
	}
	answers := setup.Answers(current, f, found)
	var askErr error
	had := answers.App.Key
	if !f.Yes {
		askErr = setup.Ask(&answers, gh.User, open, in, out, piped)
		if askErr == nil {
			askErr = takeKey(&answers, filepath.Dir(path), out)
		}
	}
	taken := ""
	if had == "" && answers.App.Key != "" {
		taken = answers.App.Key
	}
	answers, checkErr := setup.Check(answers, current, gh)
	if taken != "" && answers.App.Key == "" {
		answers.App.Key = taken
	}
	if checkErr == nil {
		checkErr = install(&answers, gh, !f.Yes, out)
	}
	checkErr = errors.Join(askErr, checkErr)
	if err := settings.Save(path, answers); err != nil {
		fmt.Fprintln(errOut, "fleet setup:", err)
		return 1
	}
	fmt.Fprintln(out, "saved", path)
	if checkErr != nil {
		fmt.Fprintln(errOut, "fleet setup:", checkErr)
		return 1
	}
	return 0
}

func takeKey(a *settings.Settings, dir string, out io.Writer) error {
	if a.App.Key != "" || a.App.ID <= 0 || a.App.Slug == "" {
		return nil
	}
	start, page := time.Now(), setup.SettingsPage(a.App.Slug)
	fmt.Fprintf(out, "This machine needs its own key: on %s, under Credentials, Key pairs, click New key.\n", page)
	fmt.Fprintf(out, "Waiting up to %s for it in %s…\n", keyWait, downloads())
	open(page)
	key, err := setup.PickUpKey(downloads(), dir, start, keyWait, time.Second)
	if key != "" {
		fmt.Fprintln(out, "kept the key, readable only by you, at", key)
		a.App.Key = key
	}
	return err
}

func appJWT(key string, id int64) (string, error) {
	f, err := os.Open(key)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("others can read it; run chmod 600 %s", key)
	}
	pemBytes, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return github.JWT(id, pemBytes, time.Now())
}

func signIn(c *github.Client) setup.SignedApp {
	return func(key string, id int64) (string, string, error) {
		jwt, err := appJWT(key, id)
		if err != nil {
			return "", "", err
		}
		app, err := c.App(jwt)
		return app.Slug, app.Owner, err
	}
}

func installation(c *github.Client) func(string, int64, string) (int64, error) {
	return func(key string, id int64, user string) (int64, error) {
		jwt, err := appJWT(key, id)
		if err != nil {
			return 0, err
		}
		return c.Installation(jwt, user)
	}
}

func install(a *settings.Settings, gh setup.GitHub, wait bool, out io.Writer) error {
	if a.App.Key == "" || a.App.ID <= 0 || a.GitHub.User == "" || gh.Installation == nil {
		return nil
	}
	page := setup.SettingsPage(a.App.Slug)
	deadline := time.Now().Add(installWait)
	for asked := false; ; time.Sleep(installEvery) {
		id, err := gh.Installation(a.App.Key, a.App.ID, a.GitHub.User)
		if err == nil {
			a.App.Installation = id
			return nil
		}
		late := !wait || time.Now().After(deadline)
		if !errors.Is(err, github.ErrNotInstalled) {
			if late || !asked {
				return err
			}
			continue
		}
		a.App.Installation = 0
		if late {
			return fmt.Errorf("the app isn't installed on your account: on %s, click Install App, then rerun setup", page)
		}
		if !asked {
			fmt.Fprintf(out, "On %s, click Install App; waiting up to %s…\n", page, installWait)
			open(page)
			asked = true
		}
	}
}

func doctorCmd(out, errOut io.Writer) int {
	var s settings.Settings
	checks := []doctor.Check{}
	if dir, err := os.UserConfigDir(); err != nil {
		checks = append(checks, doctor.Settings("", false, err))
	} else {
		path := settings.Path(dir)
		loaded, found, err := settings.Load(path)
		s = loaded
		checks = append(checks, doctor.Settings(path, found, err))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(errOut, "fleet doctor:", err)
		return 2
	}
	checks = append(checks, doctor.Mac(home, s.GitHub.User)...)
	return report(checks, out)
}

func report(checks []doctor.Check, out io.Writer) int {
	code := 0
	for _, c := range checks {
		fmt.Fprintln(out, doctor.Line(c))
		if !c.Pass {
			code = 1
		}
	}
	return code
}
