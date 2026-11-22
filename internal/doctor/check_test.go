package doctor

import "testing"

// pmset -g on a Mac host, 2026-11-19
var pmset = " powernap             0\n disksleep            0\n sleep                0\n displaysleep         10\n womp                 1\n"

func TestToolPassesWhenFoundAndNamesTheInstallWhenNot(t *testing.T) {
	if !Tool("git", "/usr/bin/git", "xcode-select --install").Pass {
		t.Error("an installed tool failed")
	}
	missing := Tool("Go", "", "brew install go")
	if missing.Pass || missing.Fix != "brew install go" {
		t.Errorf("missing tool = %+v; want a failure with its install", missing)
	}
}

func TestNoSleepPassesAMacThatNeverSleepsWhateverTheDisplayDoes(t *testing.T) {
	if !NoSleep(pmset).Pass {
		t.Error("a Mac with sleep 0 failed")
	}
}

func TestNoSleepFailsAMacThatSleepsAndNamesTheFix(t *testing.T) {
	for _, out := range []string{" sleep                1\n", "", " displaysleep 0\n"} {
		c := NoSleep(out)
		if c.Pass || c.Fix != "sudo pmset -a sleep 0" {
			t.Errorf("NoSleep(%q) = %+v; want a failure with the pmset fix", out, c)
		}
	}
}

func TestTailscalePassesWhenRunningAndOnline(t *testing.T) {
	if !Tailscale(`{"BackendState":"Running","Self":{"Online":true}}`).Pass {
		t.Error("a running, online Tailscale failed")
	}
}

func TestTailscaleFailsWhenStoppedOrNotJSON(t *testing.T) {
	for _, out := range []string{`{"BackendState":"Stopped","Self":{"Online":false}}`, `{"BackendState":"Running","Self":{"Online":false}}`, "command not found"} {
		if Tailscale(out).Pass {
			t.Errorf("Tailscale(%q) passed", out)
		}
	}
}

func TestFindTailscaleUsesThePathThenTheApp(t *testing.T) {
	if got := FindTailscale("/opt/homebrew/bin/tailscale", true); got != "/opt/homebrew/bin/tailscale" {
		t.Errorf("on the path: %q", got)
	}
	if got := FindTailscale("", true); got != TailscaleApp {
		t.Errorf("app only: %q", got)
	}
	if got := FindTailscale("", false); got != "" {
		t.Errorf("neither: %q", got)
	}
}

func TestSettingsFailsWithTheSetupFixWhenThereAreNone(t *testing.T) {
	if !Settings("/cfg/fleet/settings.toml", true, nil).Pass {
		t.Error("found settings failed")
	}
	c := Settings("/cfg/fleet/settings.toml", false, nil)
	if c.Pass || c.Fix != "fleet setup" {
		t.Errorf("no settings = %+v; want the fleet setup fix", c)
	}
}

func TestLineShowsATickOrACrossWithItsFix(t *testing.T) {
	if got := Line(Check{Name: "git", Pass: true}); got != "✓ git" {
		t.Errorf("pass: %q", got)
	}
	if got := Line(Check{Name: "Go", Fix: "brew install go"}); got != "✗ Go — fix: brew install go" {
		t.Errorf("fail: %q", got)
	}
}
