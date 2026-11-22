// https://ss64.com/mac/pmset.html
// https://tailscale.com/kb/1080/cli#status
package doctor

import (
	"encoding/json"
	"regexp"
)

type Check struct {
	Name string
	Pass bool
	Fix  string
}

const TailscaleApp = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"

var sleepLine = regexp.MustCompile(`(?m)^\s*sleep\s+(\d+)`)

func Tool(name, path, fix string) Check {
	return Check{name, path != "", fix}
}

func NoSleep(pmset string) Check {
	m := sleepLine.FindStringSubmatch(pmset)
	return Check{"never sleeps", m != nil && m[1] == "0", "sudo pmset -a sleep 0"}
}

func Tailscale(statusJSON string) Check {
	var status struct {
		BackendState string
		Self         struct{ Online bool }
	}
	running := json.Unmarshal([]byte(statusJSON), &status) == nil &&
		status.BackendState == "Running" && status.Self.Online
	return Check{"Tailscale up", running, "open Tailscale and sign in"}
}

func FindTailscale(onPath string, appInstalled bool) string {
	if onPath != "" {
		return onPath
	}
	if appInstalled {
		return TailscaleApp
	}
	return ""
}

func Settings(path string, found bool, err error) Check {
	switch {
	case err != nil:
		return Check{"settings (" + err.Error() + ")", false, "fleet setup"}
	case !found:
		return Check{"settings (none yet at " + path + ")", false, "fleet setup"}
	}
	return Check{"settings", true, ""}
}

func Line(c Check) string {
	if c.Pass {
		return "✓ " + c.Name
	}
	return "✗ " + c.Name + " — fix: " + c.Fix
}
