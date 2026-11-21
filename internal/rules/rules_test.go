package rules

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportIsFromHomeWithEachSpaceEscaped(t *testing.T) {
	got := Import("/Users/me", "/Users/me/Library/Application Support/fleet/ci/agents/rules.md")
	if got != `@~/Library/Application\ Support/fleet/ci/agents/rules.md` {
		t.Fatalf("Import = %q", got)
	}
	if got := Import("/Users/me", "/opt/fleet/ci/agents/rules.md"); got != "@/opt/fleet/ci/agents/rules.md" {
		t.Fatalf("outside home = %q", got)
	}
}

func TestAddImportSwapsTheWorkClonesImportAndKeepsTheRest(t *testing.T) {
	imp := `@~/Library/Application\ Support/fleet/ci/agents/rules.md`
	for text, want := range map[string]string{
		"":                    imp + "\n",
		"# mine\n- keep this": "# mine\n- keep this\n" + imp + "\n",
		"# mine\n@~/dev/ci/agents/rules.md\n- end": "# mine\n- end\n" + imp + "\n",
		"# mine\n" + imp + "\n":                    "# mine\n" + imp + "\n",
	} {
		if got := AddImport(text, imp); got != want {
			t.Errorf("AddImport(%q) = %q; want %q", text, got, want)
		}
	}
}

type fakeGit struct {
	ran    []string
	status string
	branch string
	top    string
	fail   error
}

func (g *fakeGit) run(name string, args ...string) (string, error) {
	g.ran = append(g.ran, strings.Join(args, " "))
	if g.fail != nil {
		return "fatal: no network", g.fail
	}
	if args[0] == "clone" {
		dir := args[len(args)-1]
		os.MkdirAll(filepath.Join(dir, "agents"), 0o755)
		os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
		os.WriteFile(filepath.Join(dir, "agents", "rules.md"), []byte("# Rules\n"), 0o644)
		os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref\n"), 0o644)
	}
	switch strings.Join(args[2:], " ") {
	case "status --porcelain":
		return g.status, nil
	case "rev-parse --abbrev-ref HEAD":
		return g.branch + "\n", nil
	case "rev-parse --show-toplevel":
		if g.top != "" {
			return g.top + "\n", nil
		}
		return args[1] + "\n", nil
	}
	return "", nil
}

func TestInstallClonesTheCopyAndPointsClaudeAndCodexAtIt(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "CLAUDE.md"), []byte("# mine\n@~/dev/ci/agents/rules.md\n"), 0o644)
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	os.Symlink(filepath.Join(home, "dev", "ci", "agents", "rules.md"), filepath.Join(home, ".codex", "AGENTS.md"))
	g := &fakeGit{branch: "dev"}
	var out bytes.Buffer
	if err := Install(home, cfg, g.run, &out); err != nil {
		t.Fatal(err)
	}
	dir, file := Dir(cfg), File(cfg)
	if g.ran[0] != "clone --quiet --branch dev --single-branch "+Repo+" "+dir {
		t.Fatalf("ran %q", g.ran)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o444 {
		t.Fatalf("rules.md: %v, %v; want read-only", info, err)
	}
	if info, _ := os.Stat(filepath.Join(dir, ".git", "HEAD")); info.Mode().Perm() != 0o644 {
		t.Fatalf(".git changed: %v", info.Mode())
	}
	claude, _ := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if string(claude) != "# mine\n"+Import(home, file)+"\n" {
		t.Fatalf("CLAUDE.md = %q", claude)
	}
	if target, _ := os.Readlink(filepath.Join(home, ".codex", "AGENTS.md")); target != file {
		t.Fatalf("Codex links %q", target)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "AGENTS.md.before-fleet")); err == nil {
		t.Fatal("backed up a link")
	}
}

func TestInstallKeepsCodexsOwnFileAndUpdatesACleanCopy(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	os.WriteFile(filepath.Join(home, ".codex", "AGENTS.md"), []byte("mine"), 0o644)
	g := &fakeGit{branch: "dev"}
	var out bytes.Buffer
	if err := Install(home, cfg, g.run, &out); err != nil {
		t.Fatal(err)
	}
	if kept, _ := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md.before-fleet")); string(kept) != "mine" || !strings.Contains(out.String(), "before-fleet") {
		t.Fatalf("kept %q; said %q", kept, out.String())
	}
	g.ran = nil
	if err := Install(home, cfg, g.run, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(g.ran, "\n"), "pull --quiet --ff-only") {
		t.Fatalf("rerun ran %q", g.ran)
	}
}

func TestInstallRefusesACopySomeoneChanged(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	g := &fakeGit{branch: "dev"}
	Install(home, cfg, g.run, &bytes.Buffer{})
	for _, bad := range []fakeGit{{branch: "dev", status: " M agents/rules.md\n"}, {branch: "someone/hq-1"}, {branch: "dev", top: home}} {
		g := bad
		if err := Install(home, cfg, g.run, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), Dir(cfg)) || strings.Contains(strings.Join(g.ran, " "), "pull") {
			t.Errorf("%+v: %v, ran %q", bad, err, g.ran)
		}
	}
	g = &fakeGit{fail: errors.New("exit status 128")}
	if err := Install(t.TempDir(), t.TempDir(), g.run, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "no network") {
		t.Fatalf("clone failure: %v", err)
	}
}

func TestInstallKeepsALinkToTheUsersOwnFile(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(home, ".codex"), 0o755)
	mine := filepath.Join(home, "dotfiles", "AGENTS.md")
	os.Symlink(mine, filepath.Join(home, ".codex", "AGENTS.md"))
	g := &fakeGit{branch: "dev"}
	if err := Install(home, cfg, g.run, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if target, _ := os.Readlink(filepath.Join(home, ".codex", "AGENTS.md.before-fleet")); target != mine {
		t.Fatalf("backup links %q", target)
	}
}
