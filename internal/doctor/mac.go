// https://pkg.go.dev/os/exec#LookPath
// https://pkg.go.dev/os/exec#Cmd.Output
package doctor

import (
	"os"
	"os/exec"
)

func find(name string) string {
	path, _ := exec.LookPath(name)
	return path
}

func output(name string, args ...string) string {
	out, _ := exec.Command(name, args...).Output()
	return string(out)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Mac() []Check {
	checks := []Check{
		Tool("Homebrew", find("brew"), "see https://brew.sh"),
		Tool("git", find("git"), "xcode-select --install"),
		Tool("gh", find("gh"), "brew install gh"),
		Tool("Go", find("go"), "brew install go"),
		Tool("Claude Code", find("claude"), "see https://claude.com/product/claude-code"),
		Tool("T3 Code", find("t3"), "curl -fsSL https://t3.codes/install.sh | sh"),
	}
	if ts := FindTailscale(find("tailscale"), exists(TailscaleApp)); ts != "" {
		checks = append(checks, Tailscale(output(ts, "status", "--json")))
	} else {
		checks = append(checks, Check{"Tailscale up", false, "install Tailscale: https://tailscale.com/download/mac"})
	}
	return append(checks, NoSleep(output("pmset", "-g")))
}
