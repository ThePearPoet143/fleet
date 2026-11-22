// https://github.com/pingdotgg/t3code/blob/main/docs/user/background-service.md
package doctor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const atTheMac = ", at the Mac (not over SSH)"

var (
	t3Status    = regexp.MustCompile(`(?m)^\s*Status:\s*(.+)$`)
	t3Installed = regexp.MustCompile(`^installed\b`)
	t3Behind    = regexp.MustCompile(`\[restart-pending\]|needs an update or repair`)
	t3Next      = regexp.MustCompile("Next:\\s*Run\\s+`([^`]+)`")
	t3Lan       = regexp.MustCompile(`(?m)^\s*Local network\s+on\b`)
	t3Port      = regexp.MustCompile(`(?m)^[ \t]*T3CODE_PORT[ \t]*=[ \t]*(\d+)`)
)

func T3Service(status string) Check {
	state := ""
	if m := t3Status.FindStringSubmatch(status); m != nil {
		state = strings.TrimSpace(m[1])
	}
	healthy := t3Installed.MatchString(state) && !t3Behind.MatchString(status)
	repair := "t3 service install"
	if m := t3Next.FindStringSubmatch(status); m != nil {
		repair = m[1]
	} else if strings.Contains(status, "[restart-pending]") {
		repair = "t3 service restart"
	}
	if state == "" {
		state = "no status"
	}
	return Check{"T3 Code service (" + state + ")", healthy, repair + atTheMac}
}

func T3Network(status string) Check {
	return Check{"T3 Code open to the network (LAN and tailnet)", t3Lan.MatchString(status), "t3 up --lan" + atTheMac}
}

func T3Port(serviceEnv string) int {
	if m := t3Port.FindStringSubmatch(serviceEnv); m != nil {
		if port, err := strconv.Atoi(m[1]); err == nil {
			return port
		}
	}
	return 3773
}

func T3Answers(statusCode, port int) Check {
	return Check{fmt.Sprintf("T3 Code answers on port %d", port), statusCode >= 200 && statusCode < 400, "t3 service restart" + atTheMac}
}

// T3Env is T3's service.env, or "".
func T3Env(home string) string { return read(filepath.Join(home, ".t3", "service.env")) }

// T3ServiceNow reads t3 service status.
func T3ServiceNow() Check { return T3Service(combined("t3", "service", "status")) }

// T3NetworkNow reads t3 status.
func T3NetworkNow() Check { return T3Network(combined("t3", "status")) }

// T3AnswersNow asks T3 on the port its service.env names.
func T3AnswersNow(home string) Check {
	port := T3Port(T3Env(home))
	return T3Answers(statusCode(fmt.Sprintf("http://127.0.0.1:%d/", port)), port)
}
