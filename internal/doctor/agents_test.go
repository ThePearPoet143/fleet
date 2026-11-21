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
