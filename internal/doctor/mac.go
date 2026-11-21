// https://pkg.go.dev/os/exec#LookPath
// https://pkg.go.dev/os/exec#Cmd.Output
// https://pkg.go.dev/os/exec#Cmd.CombinedOutput
// https://pkg.go.dev/os#Readlink
// https://pkg.go.dev/os#Stat
package doctor

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func find(name string) string {
	path, _ := exec.LookPath(name)
	return path
}

func output(name string, args ...string) string {
	out, _ := exec.Command(name, args...).Output()
	return string(out)
}

func combined(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func read(path string) string {
	text, _ := os.ReadFile(path)
	return string(text)
}

func statusCode(url string) int {
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

func Mac(home, hosts, jobBin string) []Check {
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
	if find("t3") != "" {
		env := read(filepath.Join(home, ".t3", "service.env"))
		port := T3Port(env)
		checks = append(checks,
			T3Service(combined("t3", "service", "status")),
			T3Network(combined("t3", "status")),
			T3Answers(statusCode(fmt.Sprintf("http://127.0.0.1:%d/", port)), port),
			AgentEnv(env))
	}
	checks = append(checks, token(hosts),
		TokenJob(output("launchctl", "print", fmt.Sprintf("gui/%d/dev.fleet.agent-token", os.Getuid())), jobBin),
		NoSleep(output("pmset", "-g")))
	return append(checks, devFolder(home), sharedRules(home))
}

func token(hosts string) Check {
	if hosts == "" {
		return Check{"agents' GitHub token fresh (no app installation in settings)", false, "fleet setup"}
	}
	info, err := os.Stat(hosts)
	if err != nil {
		return AgentToken(false, time.Now(), time.Now())
	}
	return AgentToken(true, info.ModTime(), time.Now())
}

func devFolder(home string) Check {
	dev := filepath.Join(home, "dev")
	entries, err := os.ReadDir(dev)
	var names []string
	for _, e := range entries {
		if info, statErr := os.Stat(filepath.Join(dev, e.Name())); statErr == nil && info.IsDir() {
			names = append(names, e.Name())
		}
	}
	return DevFolder(err == nil, names)
}

func sharedRules(home string) Check {
	ci := filepath.Join(home, "dev", "ci")
	behind := 1
	if exists(ci) {
		exec.Command("git", "-C", ci, "fetch", "-q", "origin", "dev").Run()
		if n, err := strconv.Atoi(strings.TrimSpace(output("git", "-C", ci, "rev-list", "--count", "HEAD..origin/dev"))); err == nil {
			behind = n
		}
	}
	target, _ := os.Readlink(filepath.Join(home, ".codex", "AGENTS.md"))
	return SharedRules(read(filepath.Join(home, ".claude", "CLAUDE.md")), target, filepath.Join(ci, "agents", "rules.md"), behind)
}
