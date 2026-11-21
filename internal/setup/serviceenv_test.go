package setup

import (
	"strings"
	"testing"
)

const ghDir = "/Users/me/Library/Application Support/fleet/gh"

func TestBotLinesAreTheFiveT3Needs(t *testing.T) {
	want := `GH_CONFIG_DIR="/Users/me/Library/Application Support/fleet/gh"
GIT_AUTHOR_NAME=someone-agents[bot]
GIT_AUTHOR_EMAIL=7+someone-agents[bot]@users.noreply.github.com
GIT_COMMITTER_NAME=someone-agents[bot]
GIT_COMMITTER_EMAIL=7+someone-agents[bot]@users.noreply.github.com
`
	if got, _ := MergeEnv("", BotLines(ghDir, "someone-agents", 7)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMergeEnvKeepsOtherLinesAndReplacesOldValuesInPlace(t *testing.T) {
	old := "# mine\nT3CODE_LAN=true\nGH_CONFIG_DIR=/Users/me/.people/owner/agents/gh\nGIT_AUTHOR_NAME=old[bot]\nGIT_AUTHOR_NAME=twice[bot]\nT3CODE_PORT=4000"
	got, changed := MergeEnv(old, BotLines(ghDir, "someone-agents", 7))
	if !changed || !strings.HasPrefix(got, "# mine\nT3CODE_LAN=true\nGH_CONFIG_DIR=\"/Users/me/Library/Application Support/fleet/gh\"\nGIT_AUTHOR_NAME=someone-agents[bot]\nT3CODE_PORT=4000\n") {
		t.Fatalf("merged %v:\n%s", changed, got)
	}
	if strings.Count(got, "GIT_AUTHOR_NAME") != 1 || !strings.Contains(got, "GIT_COMMITTER_EMAIL=7+someone-agents[bot]@users.noreply.github.com\n") {
		t.Fatalf("merged:\n%s", got)
	}
	if again, changed := MergeEnv(got, BotLines(ghDir, "someone-agents", 7)); changed || again != got {
		t.Fatalf("second merge changed it:\n%s", again)
	}
}

func TestMergeEnvReplacesAnExportLine(t *testing.T) {
	got, _ := MergeEnv("export GIT_AUTHOR_NAME=old\n", BotLines(ghDir, "someone-agents", 7))
	if strings.Contains(got, "old") || strings.Count(got, "GIT_AUTHOR_NAME") != 1 {
		t.Fatalf("merged:\n%s", got)
	}
}
