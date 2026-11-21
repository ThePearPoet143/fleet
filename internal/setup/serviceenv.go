// https://github.com/pingdotgg/t3code/blob/main/docs/user/background-service.md#configure-the-service-environment
// https://nodejs.org/api/util.html#utilparseenvcontent (T3 reads service.env with it)
package setup

import (
	"fmt"
	"regexp"
	"strings"
)

type EnvLine struct{ Key, Value string }

var envKey = regexp.MustCompile(`^[ \t]*(?:export[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*=`)

// BotLines start every agent T3 runs as the app's bot: gh's login and git's name and email.
func BotLines(ghDir, slug string, botID int64) []EnvLine {
	name := slug + "[bot]"
	email := fmt.Sprintf("%d+%s@users.noreply.github.com", botID, name)
	return []EnvLine{{"GH_CONFIG_DIR", ghDir}, {"GIT_AUTHOR_NAME", name}, {"GIT_AUTHOR_EMAIL", email},
		{"GIT_COMMITTER_NAME", name}, {"GIT_COMMITTER_EMAIL", email}}
}

func quote(v string) string {
	if strings.ContainsAny(v, " #'\"`\t") {
		return `"` + v + `"`
	}
	return v
}

// MergeEnv sets each line in a service.env, in place of an old value, keeping every other line.
func MergeEnv(text string, lines []EnvLine) (string, bool) {
	want := map[string]string{}
	for _, l := range lines {
		want[l.Key] = l.Key + "=" + quote(l.Value)
	}
	done := map[string]bool{}
	var out []string
	if text != "" {
		for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			m := envKey.FindStringSubmatch(line)
			if m == nil || want[m[1]] == "" {
				out = append(out, line)
			} else if !done[m[1]] {
				out, done[m[1]] = append(out, want[m[1]]), true
			}
		}
	}
	for _, l := range lines {
		if !done[l.Key] {
			out = append(out, want[l.Key])
		}
	}
	merged := strings.Join(out, "\n") + "\n"
	return merged, merged != text
}
