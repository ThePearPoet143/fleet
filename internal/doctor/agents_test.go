package doctor

import (
	"strings"
	"testing"
	"time"
)

const goodEnv = `T3CODE_LAN=true
GH_CONFIG_DIR=/Users/me/.people/owner/agents/gh
GIT_AUTHOR_NAME=someone-agents[bot]
GIT_AUTHOR_EMAIL=1+someone-agents[bot]@users.noreply.github.com
GIT_COMMITTER_NAME=someone-agents[bot]
GIT_COMMITTER_EMAIL=1+someone-agents[bot]@users.noreply.github.com
`

func TestAgentEnvPassesWhenT3StartsEveryAgentAsTheBot(t *testing.T) {
	if c := AgentEnv(goodEnv); !c.Pass {
		t.Errorf("good = %+v", c)
	}
}

func TestAgentEnvNamesWhatIsMissingAndRefusesATildePath(t *testing.T) {
	if c := AgentEnv("T3CODE_LAN=true\n"); c.Pass || !strings.Contains(c.Name, "GH_CONFIG_DIR") {
		t.Errorf("missing = %+v", c)
	}
	tilde := strings.Replace(goodEnv, "/Users/me", "~", 1)
	if c := AgentEnv(tilde); c.Pass || !strings.Contains(c.Name, "a full path, not ~") {
		t.Errorf("tilde = %+v", c)
	}
}

func TestAgentTokenPassesOnlyATokenWrittenInTheLastHour(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	if !AgentToken(true, now.Add(-20*time.Minute), now).Pass {
		t.Error("fresh failed")
	}
	if AgentToken(true, now.Add(-70*time.Minute), now).Pass || AgentToken(false, now, now).Pass {
		t.Error("old or missing passed")
	}
	if c := AgentToken(false, now, now); c.Fix != "fleet token" {
		t.Errorf("fix = %q", c.Fix)
	}
}

func TestDevFolderPassesOnlyADevFolderHoldingFleet(t *testing.T) {
	if !DevFolder(true, []string{"ci", "fleet", "homelab"}).Pass {
		t.Error("with fleet failed")
	}
	if DevFolder(true, []string{"ci"}).Pass {
		t.Error("without fleet passed")
	}
	if c := DevFolder(false, nil); c.Pass || !strings.Contains(c.Fix, "gh repo clone") {
		t.Errorf("missing = %+v", c)
	}
}

func TestSharedRulesPassesWhenClaudeImportsCodexLinksAndCiIsCurrent(t *testing.T) {
	rules := "/Users/me/dev/ci/agents/rules.md"
	if c := SharedRules("# mine\n@~/dev/ci/agents/rules.md\n", rules, rules, 0); !c.Pass {
		t.Errorf("good = %+v", c)
	}
}

func TestSharedRulesNamesEachGap(t *testing.T) {
	c := SharedRules("# mine\n", "", "/Users/me/dev/ci/agents/rules.md", 2)
	for _, gap := range []string{"Claude", "Codex", "2 behind"} {
		if c.Pass || !strings.Contains(c.Name, gap) {
			t.Errorf("SharedRules = %+v; want it to name %q", c, gap)
		}
	}
}

func TestAgentEnvTreatsAnEmptyValueAsMissingNotTheNextLine(t *testing.T) {
	env := strings.Replace(goodEnv, "GIT_AUTHOR_EMAIL=1+someone-agents[bot]@users.noreply.github.com", "GIT_AUTHOR_EMAIL=", 1)
	if c := AgentEnv(env); c.Pass || !strings.Contains(c.Name, "missing: GIT_AUTHOR_EMAIL)") {
		t.Errorf("empty email = %+v", c)
	}
}

// launchctl print gui/501/dev.fleet.agent-token on a Mac host, 2026-11-20 (fleet-ps's job)
const psJob = "gui/501/dev.fleet.agent-token = {\n\tactive count = 0\n\tpath = /Users/me/Library/LaunchAgents/dev.fleet.agent-token.plist\n\ttype = LaunchAgent\n\tstate = not running\n\n\tprogram = /opt/homebrew/bin/pwsh\n\targuments = {\n\t\t/opt/homebrew/bin/pwsh\n\t\t-NoProfile\n\t\t-File\n\t\t/Users/me/.people/owner/agents/bin/Update-AgentToken.ps1\n\t}\n"

func TestTokenJobPassesOnlyAJobRunningFleetToken(t *testing.T) {
	bin := "/Users/me/Library/Application Support/fleet/bin/fleet"
	good := "gui/501/dev.fleet.agent-token = {\n\tstate = not running\n\n\tprogram = " + bin + "\n\targuments = {\n\t\t" + bin + "\n\t\ttoken\n\t}\n"
	if c := TokenJob(good, bin); !c.Pass {
		t.Errorf("good = %+v", c)
	}
	for name, out := range map[string]string{"fleet-ps's": psJob, "none": "Bad request.\nCould not find service \"dev.fleet.agent-token\" in domain for user gui: 501\n"} {
		if c := TokenJob(out, bin); c.Pass || !strings.Contains(c.Fix, "fleet setup") {
			t.Errorf("%s = %+v", name, c)
		}
	}
}
