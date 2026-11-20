// https://code.claude.com/docs/en/memory#import-additional-files
// https://developers.openai.com/codex/guides/agents-md
package doctor

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

const RulesImport = "@~/dev/ci/agents/rules.md"

var envLine = regexp.MustCompile(`(?m)^\s*([A-Z_]+)\s*=\s*(.*)$`)

func AgentEnv(serviceEnv string) Check {
	values := map[string]string{}
	for _, m := range envLine.FindAllStringSubmatch(serviceEnv, -1) {
		values[m[1]] = strings.TrimSpace(m[2])
	}
	var missing []string
	for _, key := range []string{"GH_CONFIG_DIR", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		if values[key] == "" {
			missing = append(missing, key)
		}
	}
	if dir := values["GH_CONFIG_DIR"]; dir != "" && !strings.HasPrefix(dir, "/") {
		missing = append(missing, "GH_CONFIG_DIR (a full path, not ~)")
	}
	name := "T3 starts agents as the bot"
	if missing != nil {
		name += " (missing: " + strings.Join(missing, ", ") + ")"
	}
	return Check{name, missing == nil, "add them to ~/.t3/service.env (fleet-ps README, The agents' identity), then t3 service restart at the Mac"}
}

func AgentToken(exists bool, written, now time.Time) Check {
	return Check{"agents' GitHub token fresh", exists && now.Sub(written) < time.Hour, "pwsh -File ~/dev/fleet-ps/identity/Update-AgentToken.ps1"}
}

func DevFolder(exists bool, entries []string) Check {
	return Check{"repos in ~/dev", exists && slices.Contains(entries, "fleet"), "mkdir -p ~/dev && cd ~/dev && gh repo clone ThePearPoet143/fleet"}
}

func SharedRules(claudeMd, codexTarget, rules string, behind int) Check {
	var gaps []string
	if !slices.Contains(strings.Split(strings.ReplaceAll(claudeMd, "\r\n", "\n"), "\n"), RulesImport) {
		gaps = append(gaps, "Claude doesn't import them")
	}
	if codexTarget != rules {
		gaps = append(gaps, "Codex doesn't link them")
	}
	if behind > 0 {
		gaps = append(gaps, fmt.Sprintf("~/dev/ci is %d behind", behind))
	}
	name := "agents have ci's rules"
	if gaps != nil {
		name += " (" + strings.Join(gaps, "; ") + ")"
	}
	return Check{name, gaps == nil, "pwsh -File ~/dev/fleet-ps/rules/Install-SharedRules.ps1"}
}
