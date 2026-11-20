package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPickUpKeyMovesTheNewKeyOwnerOnly(t *testing.T) {
	downloads, dest := t.TempDir(), filepath.Join(t.TempDir(), "fleet")
	old := filepath.Join(downloads, "old.private-key.pem")
	os.WriteFile(old, []byte("old"), 0o644)
	os.Chtimes(old, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	os.WriteFile(filepath.Join(downloads, "notes.txt"), []byte("x"), 0o644)
	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(filepath.Join(downloads, "someone-agents.2026-11-20.private-key.pem"), []byte("new"), 0o644)
	}()
	key, err := PickUpKey(downloads, dest, start, time.Second, 10*time.Millisecond)
	if err != nil || key != filepath.Join(dest, "someone-agents.2026-11-20.private-key.pem") {
		t.Fatalf("PickUpKey = %q, %v", key, err)
	}
	info, err := os.Stat(key)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v, %v; want 0600", info.Mode(), err)
	}
	if _, err := os.Stat(filepath.Join(downloads, "someone-agents.2026-11-20.private-key.pem")); !os.IsNotExist(err) {
		t.Error("the download is still in Downloads")
	}
	if _, err := os.Stat(old); err != nil {
		t.Error("an older key was touched")
	}
}

func TestPickUpKeyGivesUpAfterWaiting(t *testing.T) {
	_, err := PickUpKey(t.TempDir(), t.TempDir(), time.Now(), 50*time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "no new key") {
		t.Fatalf("error = %v", err)
	}
}

func TestPickUpKeyNeverOverwritesAKey(t *testing.T) {
	downloads, dest := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dest, "a.pem"), []byte("kept"), 0o600)
	os.WriteFile(filepath.Join(downloads, "a.pem"), []byte("new"), 0o644)
	if _, err := PickUpKey(downloads, dest, time.Now().Add(-time.Minute), 50*time.Millisecond, 10*time.Millisecond); err == nil {
		t.Fatal("overwrote a key")
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "a.pem")); string(b) != "kept" {
		t.Fatal("the saved key changed")
	}
}
