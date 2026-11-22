package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ThePearPoet143/fleet/internal/doctor"
	"github.com/ThePearPoet143/fleet/internal/github"
	"github.com/ThePearPoet143/fleet/internal/settings"
	"github.com/ThePearPoet143/fleet/internal/setup"
)

func TestRunShowsUsageForAnUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"fly"}, &out, &errOut); code != 2 || errOut.String() != usage+"\n" {
		t.Fatalf("run(fly) = %d, %q; want 2 and the usage", code, errOut.String())
	}
}

func TestReportPrintsEveryLineAndFailsWhenOneFails(t *testing.T) {
	var out bytes.Buffer
	code := report([]doctor.Check{{Name: "git", Pass: true}, {Name: "Go", Fix: "brew install go"}}, &out)
	if code != 1 || out.String() != "✓ git\n✗ Go — fix: brew install go\n" {
		t.Fatalf("report = %d, %q", code, out.String())
	}
	if report([]doctor.Check{{Name: "git", Pass: true}}, &out) != 0 {
		t.Fatal("all passing should exit 0")
	}
}

func TestSetupWithFlagsAsksNothingAndSaves(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--yes"}, dir, strings.NewReader(""), setup.Detected{Tools: []string{"claude"}}, setup.GitHub{User: fake}, &out, &errOut)
	s, found, err := settings.Load(settings.Path(dir))
	if code != 0 || !found || err != nil || s.App.Slug != "someone-agents" || s.App.ID != 42 || s.Tools[0] != "claude" {
		t.Fatalf("setup = %d, %q; saved %+v, %v, %v", code, errOut.String(), s, found, err)
	}
}

func TestSetupRerunKeepsCurrentAnswers(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	key, gh := keyed(t)
	setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--purpose", "laptop", "--key", key}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if code := setupCmd(nil, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut); code != 0 {
		t.Fatalf("rerun = %d, %q", code, errOut.String())
	}
	if s, _, _ := settings.Load(settings.Path(dir)); s.Purpose != "laptop" || s.GitHub.User != "someone" || s.App.ID != 42 {
		t.Fatalf("rerun lost answers: %+v", s)
	}
}

func TestSetupKeepsValidAnswersWhenOneFails(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "nobody", "--app", "bots", "--app-id", "42", "--purpose", "laptop"}, dir, strings.NewReader(""), setup.Detected{}, setup.GitHub{User: fake}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), `no GitHub account "nobody"`) {
		t.Fatalf("setup = %d, %q", code, errOut.String())
	}
	if s, found, _ := settings.Load(settings.Path(dir)); !found || s.Purpose != "laptop" || s.App.ID != 42 || s.GitHub.User != "" {
		t.Fatalf("saved %+v, %v; want the valid answers and no user", s, found)
	}
}

func fake(name string) (string, error) {
	if name == "nobody" {
		return "", errors.New(`no GitHub account "nobody"`)
	}
	return name, nil
}

func TestPipedAnswersDriveTheForm(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	key, gh := keyed(t)
	code := setupCmd([]string{"--key", key}, dir, strings.NewReader("1\n1\nsomeone\n\n\n42\n\n"), setup.Detected{Tools: []string{"claude"}}, gh, &out, &errOut)
	s, _, _ := settings.Load(settings.Path(dir))
	if code != 0 || s.GitHub.User != "someone" || s.App.ID != 42 || s.Tools[0] != "claude" {
		t.Fatalf("setup = %d, %q; saved %+v", code, errOut.String(), s)
	}
}

func TestABadPipedAnswerFailsButKeepsTheAnswersBeforeIt(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd(nil, dir, strings.NewReader("2\n9\n"), setup.Detected{}, setup.GitHub{User: fake}, &out, &errOut)
	s, _, _ := settings.Load(settings.Path(dir))
	if code != 1 || !strings.Contains(errOut.String(), `answer "9"`) || s.Purpose != "laptop" {
		t.Fatalf("setup = %d, %q; saved %+v", code, errOut.String(), s)
	}
}

func TestSignInRefusesAKeyOthersCanRead(t *testing.T) {
	key := filepath.Join(t.TempDir(), "a.pem")
	if err := os.WriteFile(key, []byte("not a key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := signIn(nil)(key, 42); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("readable key: %v", err)
	}
	if err := os.Chmod(key, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := signIn(nil)(key, 42); err == nil || err.Error() != "the key isn't a PEM file" {
		t.Fatalf("broken key: %v", err)
	}
}

func TestSetupTakesTheNewKeyFromDownloadsAndSignsInWithIt(t *testing.T) {
	dir, dl := t.TempDir(), t.TempDir()
	useDownloads(t, dl, 2*time.Second)
	dropKey(t, dl)
	var signedWith string
	gh := setup.GitHub{User: fake, App: func(key string, id int64) (string, string, error) {
		signedWith = key
		return "someone-agents", "someone", nil
	}, Installation: func(string, int64, string) (int64, error) { return 99, nil }}
	var out, errOut bytes.Buffer
	code := setupCmd(nil, dir, strings.NewReader("1\n1\nsomeone\n\n\n42\n\n"), setup.Detected{}, gh, &out, &errOut)
	s, _, _ := settings.Load(settings.Path(dir))
	want := filepath.Join(dir, "fleet", "someone-agents.2026-11-20.private-key.pem")
	if code != 0 || s.App.Key != want || signedWith != want || !strings.Contains(out.String(), "New key") {
		t.Fatalf("setup = %d, %q; saved key %q, signed with %q\n%s", code, errOut.String(), s.App.Key, signedWith, out.String())
	}
}

func TestSetupWithYesWaitsForNoKey(t *testing.T) {
	dir := t.TempDir()
	useDownloads(t, t.TempDir(), time.Hour)
	var out, errOut bytes.Buffer
	if code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--yes"}, dir, strings.NewReader(""), setup.Detected{}, setup.GitHub{User: fake}, &out, &errOut); code != 0 {
		t.Fatalf("setup = %d, %q", code, errOut.String())
	}
}

func TestMain(m *testing.M) {
	dl, err := os.MkdirTemp("", "downloads")
	if err != nil {
		panic(err)
	}
	downloads, keyWait, installWait = func() string { return dl }, 0, 0
	agentJob = func(string) (string, error) { panic("a test installed the real job") }
	t3, err := os.MkdirTemp("", "t3")
	if err != nil {
		panic(err)
	}
	t3Home = func() string { return t3 }
	agentRules = func(string, io.Writer) error { return nil }
	code := m.Run()
	os.RemoveAll(dl)
	os.RemoveAll(t3)
	os.Exit(code)
}

func keyed(t *testing.T) (string, setup.GitHub) {
	gh := setup.GitHub{User: fake, App: func(string, int64) (string, string, error) { return "someone-agents", "someone", nil },
		Installation: func(string, int64, string) (int64, error) { return 99, nil }}
	if t == nil {
		return "", gh
	}
	t.Helper()
	key := filepath.Join(t.TempDir(), "a.pem")
	if err := os.WriteFile(key, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	return key, gh
}

func installed(after int) (setup.GitHub, *int) {
	_, gh := keyed(nil)
	calls := 0
	gh.Installation = func(key string, id int64, user string) (int64, error) {
		calls++
		if calls <= after {
			return 0, github.ErrNotInstalled
		}
		return 99, nil
	}
	return gh, &calls
}

func TestSetupSavesTheInstallation(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	gh, _ := installed(0)
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if s, _, _ := settings.Load(settings.Path(dir)); code != 0 || s.App.Installation != 99 {
		t.Fatalf("setup = %d, %q; saved %+v", code, errOut.String(), s.App)
	}
}

func TestSetupWaitsForTheAppToBeInstalled(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	gh, calls := installed(2)
	waitForInstall(t, time.Second)
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if s, _, _ := settings.Load(settings.Path(dir)); code != 0 || s.App.Installation != 99 || *calls != 3 || !strings.Contains(out.String(), "Install App") {
		t.Fatalf("setup = %d, %q, %d calls; saved %+v\n%s", code, errOut.String(), *calls, s.App, out.String())
	}
}

func TestSetupWithYesSaysWhereToInstallTheApp(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	gh, _ := installed(100)
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	s, _, _ := settings.Load(settings.Path(dir))
	if code != 1 || !strings.Contains(errOut.String(), "https://github.com/settings/apps/someone-agents") || s.App.ID != 42 || s.App.Installation != 0 {
		t.Fatalf("setup = %d, %q; saved %+v", code, errOut.String(), s.App)
	}
}

func useDownloads(t *testing.T, dir string, wait time.Duration) {
	oldDownloads, oldWait := downloads, keyWait
	downloads, keyWait = func() string { return dir }, wait
	t.Cleanup(func() { downloads, keyWait = oldDownloads, oldWait })
}

func dropKey(t *testing.T, dir string) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	b := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(filepath.Join(dir, "someone-agents.2026-11-20.private-key.pem"), b, 0o644)
	}()
}

func TestAKeyTakenThisRunIsKeptWhenSignInFails(t *testing.T) {
	dir, dl := t.TempDir(), t.TempDir()
	useDownloads(t, dl, 2*time.Second)
	dropKey(t, dl)
	gh := setup.GitHub{User: fake, App: func(string, int64) (string, string, error) { return "", "", errors.New("network down") }}
	var out, errOut bytes.Buffer
	code := setupCmd(nil, dir, strings.NewReader("1\n1\nsomeone\n\n\n42\n\n"), setup.Detected{}, gh, &out, &errOut)
	s, _, _ := settings.Load(settings.Path(dir))
	if code != 1 || s.App.Key != filepath.Join(dir, "fleet", "someone-agents.2026-11-20.private-key.pem") {
		t.Fatalf("setup = %d, %q; saved %+v", code, errOut.String(), s.App)
	}
}

func waitForInstall(t *testing.T, wait time.Duration) {
	oldWait, oldEvery := installWait, installEvery
	installWait, installEvery = wait, time.Millisecond
	t.Cleanup(func() { installWait, installEvery = oldWait, oldEvery })
}

func TestABriefErrorWhileWaitingIsRetried(t *testing.T) {
	waitForInstall(t, time.Second)
	calls := 0
	gh := setup.GitHub{Installation: func(string, int64, string) (int64, error) {
		calls++
		switch calls {
		case 1:
			return 0, github.ErrNotInstalled
		case 2:
			return 0, errors.New("GitHub answered 502 Bad Gateway")
		}
		return 99, nil
	}}
	a := settings.Settings{GitHub: settings.GitHub{User: "someone"}, App: settings.App{ID: 42, Slug: "s", Key: "/k.pem"}}
	if err := install(&a, gh, true, &bytes.Buffer{}); err != nil || a.App.Installation != 99 {
		t.Fatalf("install = %v, %+v", err, a.App)
	}
}

func TestAnErrorOtherThanNotInstalledKeepsTheSavedInstallation(t *testing.T) {
	gh := setup.GitHub{Installation: func(string, int64, string) (int64, error) { return 0, errors.New("timeout") }}
	a := settings.Settings{GitHub: settings.GitHub{User: "someone"}, App: settings.App{ID: 42, Installation: 99, Slug: "s", Key: "/k.pem"}}
	if err := install(&a, gh, false, &bytes.Buffer{}); err == nil || a.App.Installation != 99 {
		t.Fatalf("install = %v, %+v", err, a.App)
	}
}

func TestTokenWritesTheBotsLoginWithoutPrintingIt(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	s := settings.Settings{GitHub: settings.GitHub{User: "someone"}, App: settings.App{ID: 42, Installation: 99, Slug: "someone-agents", Key: key}}
	if err := settings.Save(settings.Path(dir), s); err != nil {
		t.Fatal(err)
	}
	var asked [2]any
	issue := func(k string, id, inst int64) (string, error) {
		asked = [2]any{k, inst}
		return "ghs_secret", nil
	}
	var out, errOut bytes.Buffer
	if code := tokenCmd(dir, issue, &out, &errOut); code != 0 || asked != [2]any{key, int64(99)} {
		t.Fatalf("token = %d, %q; asked %v", code, errOut.String(), asked)
	}
	hosts, err := os.ReadFile(filepath.Join(dir, "fleet", "gh", "hosts.yml"))
	if err != nil || !strings.Contains(string(hosts), "oauth_token: ghs_secret") || !strings.Contains(string(hosts), "user: someone-agents[bot]") {
		t.Fatalf("hosts.yml = %q, %v", hosts, err)
	}
	if strings.Contains(out.String()+errOut.String(), "ghs_secret") || !strings.Contains(out.String(), "token refreshed") {
		t.Fatalf("printed %q %q", out.String(), errOut.String())
	}
}

func TestTokenNeedsSetupFirstAndKeepsTheOldLogin(t *testing.T) {
	dir := t.TempDir()
	if err := settings.Save(settings.Path(dir), settings.Settings{App: settings.App{ID: 42, Slug: "someone-agents", Key: "/k.pem"}}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	issue := func(string, int64, int64) (string, error) { return "", errors.New("GitHub answered 401") }
	if code := tokenCmd(dir, issue, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "fleet setup") {
		t.Fatalf("no installation = %d, %q", code, errOut.String())
	}
	s, _, _ := settings.Load(settings.Path(dir))
	s.App.Installation = 99
	settings.Save(settings.Path(dir), s)
	errOut.Reset()
	if code := tokenCmd(dir, issue, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "failed: GitHub answered 401") {
		t.Fatalf("refused = %d, %q", code, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "fleet", "gh", "hosts.yml")); err == nil {
		t.Fatal("a failed refresh wrote a login")
	}
}

func TestSetupEndsByRefreshingTheTokenAndInstallingTheJob(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	gh, _ := installed(0)
	gh.Token = func(string, int64, int64) (string, error) { return "ghs_secret", nil }
	jobs := useJob(t, nil)
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if code != 0 || *jobs != 1 || !strings.Contains(out.String(), "token refreshed") || strings.Contains(out.String(), "ghs_secret") {
		t.Fatalf("setup = %d, %q, %d jobs\n%s", code, errOut.String(), *jobs, out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "fleet", "gh", "hosts.yml")); err != nil {
		t.Fatal(err)
	}
}

func TestSetupInstallsNoJobWithoutAWorkingToken(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	gh, _ := installed(0)
	gh.Token = func(string, int64, int64) (string, error) { return "", errors.New("GitHub answered 401") }
	bots := 0
	gh.Bot = func(string) (int64, error) { bots++; return 7, nil }
	jobs := useJob(t, nil)
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if code != 1 || *jobs != 0 || bots != 0 || !strings.Contains(errOut.String(), "401") {
		t.Fatalf("setup = %d, %q, %d jobs, %d service.env lookups", code, errOut.String(), *jobs, bots)
	}
}

func TestAJobFailureStillWritesT3sServiceEnv(t *testing.T) {
	dir, t3 := t.TempDir(), t.TempDir()
	old := t3Home
	t3Home = func() string { return t3 }
	t.Cleanup(func() { t3Home = old })
	key, _ := keyed(t)
	gh, _ := installed(0)
	gh.Token = func(string, int64, int64) (string, error) { return "ghs_secret", nil }
	gh.Bot = func(string) (int64, error) { return 7, nil }
	useJob(t, errors.New("launchctl bootstrap: 125"))
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	env, _ := os.ReadFile(filepath.Join(t3, "service.env"))
	if code != 1 || errOut.String() != "fleet setup: the token job: launchctl bootstrap: 125\n" || !strings.Contains(string(env), "GIT_AUTHOR_NAME=someone-agents[bot]") {
		t.Fatalf("setup = %d, %q; service.env %q", code, errOut.String(), env)
	}
}

func TestSetupSaysWhenLaunchdRefusesTheJob(t *testing.T) {
	dir := t.TempDir()
	key, _ := keyed(t)
	gh, _ := installed(0)
	gh.Token = func(string, int64, int64) (string, error) { return "ghs_secret", nil }
	useJob(t, errors.New("launchctl bootstrap: 125: start it at the Mac, not over SSH"))
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "not over SSH") {
		t.Fatalf("setup = %d, %q", code, errOut.String())
	}
}

func useJob(t *testing.T, err error) *int {
	old, calls := agentJob, 0
	agentJob = func(string) (string, error) {
		calls++
		return "/h/Library/LaunchAgents/dev.fleet.agent-token.plist", err
	}
	t.Cleanup(func() { agentJob = old })
	return &calls
}

func TestSetupWritesTheBotsLinesIntoT3sServiceEnv(t *testing.T) {
	dir, t3 := t.TempDir(), t.TempDir()
	old := t3Home
	t3Home = func() string { return t3 }
	t.Cleanup(func() { t3Home = old })
	os.WriteFile(filepath.Join(t3, "service.env"), []byte("T3CODE_LAN=true\n"), 0o644)
	key, _ := keyed(t)
	gh, _ := installed(0)
	gh.Token = func(string, int64, int64) (string, error) { return "ghs_secret", nil }
	gh.Bot = func(slug string) (int64, error) { return 7, nil }
	useJob(t, nil)
	args := []string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}
	var out, errOut bytes.Buffer
	if code := setupCmd(args, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut); code != 0 || !strings.Contains(out.String(), "t3 service restart") {
		t.Fatalf("setup = %d, %q\n%s", code, errOut.String(), out.String())
	}
	env, _ := os.ReadFile(filepath.Join(t3, "service.env"))
	info, _ := os.Stat(filepath.Join(t3, "service.env"))
	if !strings.HasPrefix(string(env), "T3CODE_LAN=true\nGH_CONFIG_DIR="+filepath.Join(dir, "fleet", "gh")+"\n") || !strings.Contains(string(env), "GIT_AUTHOR_EMAIL=7+someone-agents[bot]@") || info.Mode().Perm() != 0o600 {
		t.Fatalf("service.env %v:\n%s", info.Mode(), env)
	}
	out.Reset()
	if code := setupCmd(args, dir, strings.NewReader(""), setup.Detected{}, gh, &out, &errOut); code != 0 || strings.Contains(out.String(), "t3 service restart") {
		t.Fatalf("rerun = %d, %q\n%s", code, errOut.String(), out.String())
	}
}

func TestServiceEnvKeepsALinkAndSkipsAMachineWithoutT3(t *testing.T) {
	t3, dots := t.TempDir(), t.TempDir()
	old := t3Home
	t.Cleanup(func() { t3Home = old })
	bot := func(string) (int64, error) { return 7, nil }
	t3Home = func() string { return filepath.Join(t3, "none") }
	var out bytes.Buffer
	if err := serviceEnv("/cfg", "someone-agents", bot, &out); err != nil || !strings.Contains(out.String(), "skipped") {
		t.Fatalf("no T3: %v, %q", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(t3, "none")); err == nil {
		t.Fatal("made a T3 folder")
	}
	t3Home = func() string { return t3 }
	real := filepath.Join(dots, "service.env")
	os.WriteFile(real, []byte("T3CODE_LAN=true\n"), 0o600)
	os.Symlink(real, filepath.Join(t3, "service.env"))
	if err := serviceEnv("/cfg", "someone-agents", bot, &out); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(t3, "service.env")); err != nil || target != real {
		t.Fatalf("link replaced: %q, %v", target, err)
	}
	if text, _ := os.ReadFile(real); !strings.Contains(string(text), "GIT_AUTHOR_NAME=someone-agents[bot]") {
		t.Fatalf("target = %q", text)
	}
}

func TestSetupSetsUpTheRulesAndReportsAFailure(t *testing.T) {
	old := agentRules
	t.Cleanup(func() { agentRules = old })
	calls := 0
	agentRules = func(string, io.Writer) error { calls++; return errors.New("git clone: fatal: no network") }
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--yes"}, t.TempDir(), strings.NewReader(""), setup.Detected{}, setup.GitHub{User: fake}, &out, &errOut)
	if code != 1 || calls != 1 || !strings.Contains(errOut.String(), "ci's rules: git clone: fatal: no network") {
		t.Fatalf("setup = %d, %q, %d calls", code, errOut.String(), calls)
	}
}

func TestARulesFailureStillLeavesTheTokenJob(t *testing.T) {
	old := agentRules
	t.Cleanup(func() { agentRules = old })
	agentRules = func(string, io.Writer) error { return errors.New("no network") }
	key, _ := keyed(t)
	gh, _ := installed(0)
	gh.Token = func(string, int64, int64) (string, error) { return "ghs_secret", nil }
	jobs := useJob(t, nil)
	var out, errOut bytes.Buffer
	code := setupCmd([]string{"--github-user", "someone", "--app-id", "42", "--key", key, "--yes"}, t.TempDir(), strings.NewReader(""), setup.Detected{}, gh, &out, &errOut)
	if code != 1 || *jobs != 1 || !strings.Contains(errOut.String(), "ci's rules: no network") {
		t.Fatalf("setup = %d, %q, %d jobs", code, errOut.String(), *jobs)
	}
}

func withForm(t *testing.T, ask func(a *settings.Settings, user setup.Lookup, open func(string), in io.Reader, out io.Writer) error) {
	t.Helper()
	oldForm, oldAsk := form, askForm
	form, askForm = true, ask
	t.Cleanup(func() { form, askForm = oldForm, oldAsk })
}

func TestSetupInATerminalAsksInTheFormAndSaves(t *testing.T) {
	withForm(t, func(a *settings.Settings, _ setup.Lookup, _ func(string), _ io.Reader, _ io.Writer) error {
		a.Purpose, a.GitHub.User, a.App.Slug, a.App.ID = "laptop", "someone", "someone-agents", 42
		return nil
	})
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd(nil, dir, strings.NewReader("2\n"), setup.Detected{}, setup.GitHub{User: fake}, &out, &errOut)
	if s, _, _ := settings.Load(settings.Path(dir)); s.Purpose != "laptop" || s.App.ID != 42 || strings.Contains(out.String(), "Enter a number") {
		t.Fatalf("setup = %d, %q; saved %+v\n%s", code, errOut.String(), s, out.String())
	}
}

func TestStoppingTheFormSavesNothing(t *testing.T) {
	withForm(t, func(a *settings.Settings, _ setup.Lookup, _ func(string), _ io.Reader, _ io.Writer) error {
		a.Purpose = "laptop"
		return setup.ErrStopped
	})
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := setupCmd(nil, dir, strings.NewReader(""), setup.Detected{}, setup.GitHub{User: fake}, &out, &errOut)
	if _, err := os.Stat(settings.Path(dir)); code != 130 || !errors.Is(err, os.ErrNotExist) || !strings.Contains(errOut.String(), "nothing saved") {
		t.Fatalf("setup = %d, %q, settings %v", code, errOut.String(), err)
	}
}
