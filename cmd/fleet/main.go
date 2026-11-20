// https://pkg.go.dev/os#UserConfigDir
package main

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/system"
)

const usage = "usage: fleet doctor"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) != 1 || args[0] != "doctor" {
		fmt.Fprintln(errOut, usage)
		return 2
	}
	if _, err := system.Detect(runtime.GOOS); err != nil {
		fmt.Fprintln(errOut, "fleet doctor:", err)
		return 2
	}
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
	home, _ := os.UserHomeDir()
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
