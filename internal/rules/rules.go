// https://code.claude.com/docs/en/memory#import-additional-files
// https://developers.openai.com/codex/guides/agents-md
// https://git-scm.com/docs/git-pull#Documentation/git-pull.txt---ff-only
package rules

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ThePearPoet143/fleet/internal/files"
)

const (
	Repo   = "https://github.com/ThePearPoet143/ci"
	Branch = "dev"
	// Old is the import of the work clone, whose branch agents change.
	Old = "@~/dev/ci/agents/rules.md"
)

type Runner func(name string, args ...string) (string, error)

// Dir is fleet's own copy of ci, which only fleet updates.
func Dir(configDir string) string  { return filepath.Join(configDir, "fleet", "ci") }
func File(configDir string) string { return filepath.Join(Dir(configDir), "agents", "rules.md") }

// Import is the CLAUDE.md line for file: from ~ when it's in home, each space after a backslash.
func Import(home, file string) string {
	path := file
	if rel, err := filepath.Rel(home, file); err == nil && !strings.HasPrefix(rel, "..") {
		path = "~/" + filepath.ToSlash(rel)
	}
	return "@" + strings.ReplaceAll(path, " ", `\ `)
}

func lines(text string) []string {
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// AddImport drops the work clone's import and adds imp once, keeping every other line.
func AddImport(text, imp string) string {
	var out []string
	has := false
	for _, l := range lines(text) {
		switch strings.TrimSpace(l) {
		case Old:
			continue
		case imp:
			has = true
		}
		out = append(out, l)
	}
	if !has {
		out = append(out, imp)
	}
	return strings.Join(out, "\n") + "\n"
}

func git(run Runner, args ...string) (string, error) {
	out, err := run("git", args...)
	if err != nil {
		return out, fmt.Errorf("git %s: %s (%v)", args[0], strings.TrimSpace(out), err)
	}
	return out, nil
}

func update(dir string, run Runner) error {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		_, err := git(run, "clone", "--quiet", "--branch", Branch, "--single-branch", Repo, dir)
		return err
	}
	branch, err := git(run, "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	status, err := git(run, "-C", dir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(branch) != Branch || strings.TrimSpace(status) != "" {
		return fmt.Errorf("%s was changed (on %s, or edited); only fleet updates it: remove it and rerun setup", dir, strings.TrimSpace(branch))
	}
	_, err = git(run, "-C", dir, "pull", "--quiet", "--ff-only")
	return err
}

// readOnly takes write access off every file outside .git, so an edit fails instead of changing agents' rules.
func readOnly(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.Chmod(path, info.Mode().Perm()&^0o222)
		}
		return nil
	})
}

// Install updates fleet's copy of ci, then has Claude Code import its rules and Codex link them.
func Install(home, configDir string, run Runner, out io.Writer) error {
	dir, file := Dir(configDir), File(configDir)
	if err := update(dir, run); err != nil {
		return err
	}
	if err := readOnly(dir); err != nil {
		return err
	}
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("%s is missing from ci's %s", file, Branch)
	}
	claudeMd := filepath.Join(home, ".claude", "CLAUDE.md")
	if real, err := filepath.EvalSymlinks(claudeMd); err == nil {
		claudeMd = real
	}
	old, err := os.ReadFile(claudeMd)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if merged := AddImport(string(old), Import(home, file)); merged != string(old) {
		mode := os.FileMode(0o644)
		if info, err := os.Stat(claudeMd); err == nil {
			mode = info.Mode().Perm()
		}
		if err := files.Write(claudeMd, []byte(merged), mode); err != nil {
			return err
		}
		fmt.Fprintf(out, "Claude Code: %s imports fleet's copy of ci's rules\n", claudeMd)
	}
	return linkCodex(filepath.Join(home, ".codex", "AGENTS.md"), file, out)
}

func linkCodex(path, file string, out io.Writer) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		if target, _ := os.Readlink(path); target == file {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	default:
		backup := path + ".before-fleet"
		if _, err := os.Lstat(backup); err == nil {
			return fmt.Errorf("%s is your own file and %s exists: move one of them", path, backup)
		}
		if err := os.Rename(path, backup); err != nil {
			return err
		}
		fmt.Fprintf(out, "Codex: your old %s is kept as %s\n", path, backup)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(file, path); err != nil {
		return err
	}
	fmt.Fprintf(out, "Codex: %s links fleet's copy of ci's rules\n", path)
	return nil
}
