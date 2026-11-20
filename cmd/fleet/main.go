// https://pkg.go.dev/os#UserConfigDir
// https://pkg.go.dev/os#FileMode
// https://ss64.com/mac/open.html
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/github"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/setup"
	"github.com/ThePearPoet143/fleet/internal/system"
)

const usage = "usage: fleet doctor | fleet setup [--purpose host|laptop] [--use personal|shared] [--github-user name]\n" +
	"  [--app slug] [--app-id n] [--tools claude,codex] [--reach tailscale,t3,ssh] [--key /full/path.pem] [--yes]"

var open = func(url string) {}

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
		}
		if code := setupCmd(args[1:], dir, os.Stdin, setup.Detect(), github.New().User, out, errOut); code != 0 {
			return code
		}
	}
	return doctorCmd(out, errOut)
}

func setupCmd(args []string, configDir string, in io.Reader, found setup.Detected, user setup.Lookup, out, errOut io.Writer) int {
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
	if !f.Yes {
		if err := setup.Ask(&answers, user, open, in, out); err != nil {
			fmt.Fprintln(errOut, "fleet setup:", err)
			return 1
		}
	}
	answers, checkErr := setup.Check(answers, current, user)
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
