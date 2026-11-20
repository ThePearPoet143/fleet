package doctor

import (
	"strings"
	"testing"
)

// t3 service status on a Mac host, 2026-11-19
const (
	t3Broken  = "T3 Code service\n  Status: needs an update or repair\n  Unit: /Users/me/Library/LaunchAgents/com.t3tools.t3code.service.plist\n"
	t3Healthy = "T3 Code service\n  Status: installed · t3@0.1.10\n  Unit: /Users/me/Library/LaunchAgents/com.t3tools.t3code.service.plist\n"
	t3Pending = "  Status: installed · t3@0.1.10\n  [restart-pending] A newer version is installed but the service is still running the previous one.\n"
	t3Printed = "  Status: needs an update or repair\n  Next: Run `/Users/me/.t3/runtime/versions/0.1.10/t3 service install` to repair it.\n"
)

func TestT3ServiceFailsAServiceThatNeedsRepair(t *testing.T) {
	c := T3Service(t3Broken)
	if c.Pass || !strings.HasPrefix(c.Fix, "t3 service install") {
		t.Errorf("broken = %+v", c)
	}
}

func TestT3ServicePassesOneInstalledAndCurrent(t *testing.T) {
	if c := T3Service(t3Healthy); !c.Pass || c.Name != "T3 Code service (installed · t3@0.1.10)" {
		t.Errorf("healthy = %+v", c)
	}
}

func TestT3ServiceFailsAPendingRestartAndGivesT3sOwnRepair(t *testing.T) {
	if c := T3Service(t3Pending); c.Pass || !strings.HasPrefix(c.Fix, "t3 service restart") {
		t.Errorf("pending = %+v", c)
	}
	want := "/Users/me/.t3/runtime/versions/0.1.10/t3 service install, at the Mac (not over SSH)"
	if c := T3Service(t3Printed); c.Fix != want {
		t.Errorf("printed fix = %q; want %q", c.Fix, want)
	}
}

func TestT3ServiceFailsWithNoStatusLine(t *testing.T) {
	if c := T3Service("t3: command not found"); c.Pass || c.Name != "T3 Code service (no status)" {
		t.Errorf("no status = %+v", c)
	}
}

func TestT3NetworkPassesOnlyWhenTheLocalNetworkIsOn(t *testing.T) {
	if !T3Network("This machine\n  Local network  on · pairing code only\n").Pass {
		t.Error("on failed")
	}
	if c := T3Network("This machine\n  Local network  off\n"); c.Pass || !strings.HasPrefix(c.Fix, "t3 up --lan") {
		t.Errorf("off = %+v", c)
	}
}

func TestT3PortReadsServiceEnvOrUses3773(t *testing.T) {
	for env, want := range map[string]int{"T3CODE_LAN=true\nT3CODE_PORT=4000\n": 4000, "T3CODE_LAN=true\n": 3773, "": 3773} {
		if got := T3Port(env); got != want {
			t.Errorf("T3Port(%q) = %d; want %d", env, got, want)
		}
	}
}

func TestT3AnswersPassesA2xxOr3xx(t *testing.T) {
	if !T3Answers(200, 3773).Pass || T3Answers(0, 3773).Pass || T3Answers(500, 3773).Pass {
		t.Error("wrong pass")
	}
	if c := T3Answers(0, 4000); c.Name != "T3 Code answers on port 4000" || !strings.HasPrefix(c.Fix, "t3 service restart") {
		t.Errorf("down = %+v", c)
	}
}
