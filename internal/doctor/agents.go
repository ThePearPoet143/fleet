// https://code.claude.com/docs/en/memory#import-additional-files
// https://developers.openai.com/codex/guides/agents-md
package doctor

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ThePearPoet143/fleet/internal/rules"
)

var envLine = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?([A-Z_]+)[ \t]*=[ \t]*(.*)$`)

// envValue reads a value as Node's util.parseEnv does: a quoted value up to its closing quote, else up to a #.
func envValue(raw string) string {
	v := strings.TrimSpace(raw)
	if v != "" && strings.ContainsRune("\"'`", rune(v[0])) {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : 1+end]
		}
	}
	if i := strings.IndexByte(v, '#'); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// AgentEnv reads T3's service.env; with fleet's gh folder and app it also checks they're the ones named.
func AgentEnv(serviceEnv, ghDir, slug string) Check {
	values := map[string]string{}
	for _, m := range envLine.FindAllStringSubmatch(serviceEnv, -1) {
		values[m[1]] = envValue(m[2])
	}
	var missing []string
	for _, key := range []string{"GH_CONFIG_DIR", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		if values[key] == "" {
			missing = append(missing, key)
		}
	}
	if dir := values["GH_CONFIG_DIR"]; dir != "" && !strings.HasPrefix(dir, "/") {
		missing = append(missing, "GH_CONFIG_DIR (a full path, not ~)")
	} else if dir != "" && ghDir != "" && dir != ghDir {
		missing = append(missing, "GH_CONFIG_DIR isn't fleet's login, "+ghDir)
	}
	if name := values["GIT_AUTHOR_NAME"]; slug != "" && name != "" && (name != slug+"[bot]" || values["GIT_COMMITTER_NAME"] != name) {
		missing = append(missing, "commits as "+name+", not "+slug+"[bot]")
	}
	name := "T3 starts agents as the bot"
	if missing != nil {
		name += " (missing: " + strings.Join(missing, ", ") + ")"
	}
	return Check{name, missing == nil, "fleet setup, then t3 service restart" + atTheMac}
}

func AgentToken(exists bool, written, now time.Time) Check {
	return Check{"agents' GitHub token fresh", exists && now.Sub(written) < time.Hour, "fleet token"}
}

var (
	jobProgram = regexp.MustCompile(`(?m)^\s*program = (.+)$`)
	jobArgs    = regexp.MustCompile(`(?s)arguments = \{\n(.*?)\n\s*\}`)
)

// TokenJob reads launchctl print gui/<uid>/dev.fleet.agent-token.
func TokenJob(print, bin string) Check {
	ok := false
	if m := jobProgram.FindStringSubmatch(print); m != nil && strings.TrimSpace(m[1]) == bin {
		if a := jobArgs.FindStringSubmatch(print); a != nil {
			args := strings.Fields(strings.ReplaceAll(a[1], bin, ""))
			ok = len(args) == 1 && args[0] == "token"
		}
	}
	return Check{"the token job runs fleet token", ok, "fleet setup" + atTheMac}
}

func DevFolder(exists bool, entries []string) Check {
	return Check{"repos in ~/dev", exists && slices.Contains(entries, "fleet"), "mkdir -p ~/dev && cd ~/dev && gh repo clone ThePearPoet143/fleet"}
}

// SharedRules: behind is how far fleet's copy of ci is behind, or -1 when there's no copy.
func SharedRules(claudeMd, imp, codexTarget, rulesFile string, behind int) Check {
	var gaps []string
	claude := strings.Split(strings.ReplaceAll(claudeMd, "\r\n", "\n"), "\n")
	if !slices.Contains(claude, imp) {
		gaps = append(gaps, "Claude doesn't import them")
	}
	if slices.Contains(claude, rules.Old) {
		gaps = append(gaps, "Claude still imports the work clone's")
	}
	if codexTarget != rulesFile {
		gaps = append(gaps, "Codex doesn't link them")
	}
	switch {
	case behind < 0:
		gaps = append(gaps, "no rules copy")
	case behind > 0:
		gaps = append(gaps, fmt.Sprintf("the copy is %d behind", behind))
	}
	name := "agents have ci's rules"
	if gaps != nil {
		name += " (" + strings.Join(gaps, "; ") + ")"
	}
	return Check{name, gaps == nil, "fleet setup"}
}
