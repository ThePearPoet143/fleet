// https://pkg.go.dev/os#UserConfigDir
// https://pkg.go.dev/os#FileMode
// https://ss64.com/mac/open.html
// https://github.com/charmbracelet/huh/blob/v2.0.3/README.md#accessibility
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
	"github.com/ThePearPoet143/fleet/internal/files"
	"github.com/ThePearPoet143/fleet/internal/github"
	"github.com/ThePearPoet143/fleet/internal/job"
	"github.com/ThePearPoet143/fleet/internal/rules"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/setup"
	"github.com/ThePearPoet143/fleet/internal/system"
	"github.com/ThePearPoet143/fleet/internal/token"
)

const usage = "usage: fleet doctor | fleet token | fleet setup [--purpose host|laptop] [--use personal|shared] [--github-user name]\n" +
	"  [--app slug] [--app-id n] [--tools claude,codex] [--reach tailscale,t3,ssh] [--key /full/path.pem] [--yes]"

var (
	open         = func(url string) {}
	piped        = true
	form         = false
	askForm      = setup.Form
	keyWait      = 5 * time.Minute
	installWait  = 5 * time.Minute
	installEvery = 3 * time.Second
	agentJob     = installJob
	agentRules   = installRules
	t3Home       = func() string {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".t3")
	}
	downloads = func() string {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Downloads")
	}
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || (args[0] != "doctor" && args[0] != "setup" && args[0] != "token") || (args[0] != "setup" && len(args) > 1) {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	if _, err := system.Detect(runtime.GOOS); err != nil {
		fmt.Fprintf(errOut, "fleet %s: %v\n", args[0], err)
		return 2
	}
	if args[0] == "token" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintln(errOut, "fleet token:", err)
			return 2
		}
		return tokenCmd(dir, issue(github.New()), out, errOut)
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
			form = os.Getenv("ACCESSIBLE") == ""
		}
		if code := setupCmd(args[1:], dir, os.Stdin, setup.Detect(), setup.GitHub{User: github.New().User, App: signIn(github.New()), Installation: installation(github.New()), Token: issue(github.New()), Bot: github.New().Bot}, out, errOut); code != 0 {
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
		if form {
			askErr = askForm(&answers, gh.User, open, in, out)
		} else {
			askErr = setup.Ask(&answers, gh.User, open, in, out, piped)
		}
		if errors.Is(askErr, setup.ErrStopped) {
			fmt.Fprintln(errOut, "fleet setup:", askErr)
			return 1
		}
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
	code := botSteps(configDir, answers.App, gh, out, errOut)
	if err := agentRules(configDir, out); err != nil {
		fmt.Fprintln(errOut, "fleet setup: ci's rules:", err)
		code = 1
	}
	return code
}

// botSteps refresh the token, install its job and make T3 start agents as the bot, once the app is installed.
func botSteps(configDir string, app settings.App, gh setup.GitHub, out, errOut io.Writer) int {
	if gh.Token == nil || app.Installation <= 0 {
		return 0
	}
	if code := tokenCmd(configDir, gh.Token, out, errOut); code != 0 {
		return code
	}
	plist, err := agentJob(configDir)
	if err != nil {
		fmt.Fprintln(errOut, "fleet setup: the token job:", err)
		return 1
	}
	fmt.Fprintf(out, "the token job (%s) runs fleet token every %d minutes and at login\n", plist, job.Every/60)
	if gh.Bot == nil {
		return 0
	}
	if err := serviceEnv(configDir, app.Slug, gh.Bot, out); err != nil {
		fmt.Fprintln(errOut, "fleet setup: T3's service.env:", err)
		return 1
	}
	return 0
}

// installRules keeps fleet's own copy of ci, which agents' rules come from.
func installRules(configDir string, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	run := func(name string, args ...string) (string, error) {
		out, err := rules.Git(args...).CombinedOutput()
		return string(out), err
	}
	return rules.Install(home, configDir, run, out)
}

// serviceEnv makes every agent T3 starts act as the bot; T3 applies it on its next start.
func serviceEnv(configDir, slug string, bot func(string) (int64, error), out io.Writer) error {
	if _, err := os.Stat(t3Home()); err != nil {
		fmt.Fprintf(out, "T3 Code isn't set up here (no %s); skipped its service.env\n", t3Home())
		return nil
	}
	id, err := bot(slug)
	if err != nil {
		return err
	}
	path := filepath.Join(t3Home(), "service.env")
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	merged, changed := setup.MergeEnv(string(old), setup.BotLines(token.Dir(configDir), slug, id))
	if !changed {
		return os.Chmod(path, 0o600)
	}
	if err := files.Write(path, []byte(merged), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s starts agents as %s[bot]; run t3 service restart at the Mac (not over SSH) to apply it\n", path, slug)
	return nil
}

// installJob copies this program next to settings and loads the login job that runs it.
func installJob(configDir string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return "", err
	}
	bin := job.Bin(configDir)
	if err := job.Copy(self, bin); err != nil {
		return "", err
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", job.Label+".plist")
	run := func(name string, args ...string) (string, error) {
		out, err := exec.Command(name, args...).CombinedOutput()
		return string(out), err
	}
	return plist, job.Install(plist, job.Job{Bin: bin, Home: home, Log: job.Log(configDir)}, os.Getuid(), run)
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

func issue(c *github.Client) func(string, int64, int64) (string, error) {
	return func(key string, id, installation int64) (string, error) {
		jwt, err := appJWT(key, id)
		if err != nil {
			return "", err
		}
		return c.Token(jwt, installation)
	}
}

// tokenCmd refreshes the agents' gh login; it never prints the token.
func tokenCmd(configDir string, issue func(key string, id, installation int64) (string, error), out, errOut io.Writer) int {
	now := time.Now().Format(time.RFC3339)
	s, found, err := settings.Load(settings.Path(configDir))
	if err == nil && (!found || s.App.ID <= 0 || s.App.Installation <= 0 || s.App.Slug == "" || s.App.Key == "") {
		err = errors.New("settings lack the app, its key or its installation; run fleet setup")
	}
	var t string
	if err == nil {
		t, err = issue(s.App.Key, s.App.ID, s.App.Installation)
	}
	dir := token.Dir(configDir)
	if err == nil {
		err = token.Write(dir, t, s.App.Slug)
	}
	if err != nil {
		fmt.Fprintf(errOut, "%s fleet token failed: %v\n", now, err)
		return 1
	}
	fmt.Fprintf(out, "%s token refreshed in %s\n", now, filepath.Join(dir, "hosts.yml"))
	return 0
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
	hosts, jobBin, ghDir, ciCopy := "", "", "", ""
	checks := []doctor.Check{}
	if dir, err := os.UserConfigDir(); err != nil {
		checks = append(checks, doctor.Settings("", false, err))
	} else {
		path := settings.Path(dir)
		loaded, found, err := settings.Load(path)
		s = loaded
		checks = append(checks, doctor.Settings(path, found, err))
		if s.App.Installation > 0 {
			hosts = filepath.Join(token.Dir(dir), "hosts.yml")
		}
		jobBin, ghDir, ciCopy = job.Bin(dir), token.Dir(dir), rules.Dir(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(errOut, "fleet doctor:", err)
		return 2
	}
	checks = append(checks, doctor.Mac(home, hosts, jobBin, ghDir, s.App.Slug, ciCopy)...)
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
