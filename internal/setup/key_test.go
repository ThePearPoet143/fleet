package setup

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPickUpKeyMovesTheNewKeyOwnerOnly(t *testing.T) {
	downloads, dest := t.TempDir(), filepath.Join(t.TempDir(), "fleet")
	old := filepath.Join(downloads, "old.private-key.pem")
	os.WriteFile(old, []byte("old"), 0o644)
	os.Chtimes(old, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour))
	os.WriteFile(filepath.Join(downloads, "notes.txt"), []byte("x"), 0o644)
	start, key := time.Now(), rsaPEM(t)
	go func() {
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(filepath.Join(downloads, "someone-agents.2026-11-20.private-key.pem"), key, 0o644)
	}()
	got, err := PickUpKey(downloads, dest, start, time.Second, 10*time.Millisecond)
	if err != nil || got != filepath.Join(dest, "someone-agents.2026-11-20.private-key.pem") {
		t.Fatalf("PickUpKey = %q, %v", got, err)
	}
	info, err := os.Stat(got)
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

func rsaPEM(t *testing.T) []byte {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
}

func TestPickUpKeyTakesOnlyACompleteRegularKeyFile(t *testing.T) {
	downloads, dest := t.TempDir(), t.TempDir()
	start := time.Now().Add(-time.Second)
	os.WriteFile(filepath.Join(downloads, "a-empty.pem"), nil, 0o644)
	os.WriteFile(filepath.Join(downloads, "b-cert.pem"), []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"), 0o644)
	os.WriteFile(filepath.Join(t.TempDir(), "elsewhere.pem"), rsaPEM(t), 0o600)
	os.Symlink(filepath.Join(t.TempDir(), "elsewhere.pem"), filepath.Join(downloads, "c-link.pem"))
	syscall.Mkfifo(filepath.Join(downloads, "d-fifo.pem"), 0o644)
	os.WriteFile(filepath.Join(downloads, "e-real.private-key.pem"), rsaPEM(t), 0o644)
	key, err := PickUpKey(downloads, dest, start, time.Second, 10*time.Millisecond)
	if err != nil || filepath.Base(key) != "e-real.private-key.pem" {
		t.Fatalf("PickUpKey = %q, %v", key, err)
	}
	for _, left := range []string{"a-empty.pem", "b-cert.pem", "c-link.pem", "d-fifo.pem"} {
		if _, err := os.Lstat(filepath.Join(downloads, left)); err != nil {
			t.Errorf("%s was taken or removed", left)
		}
	}
}

func TestASecondKeyWithTheSameNameIsKeptBesideTheFirst(t *testing.T) {
	downloads, dest := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dest, "a.private-key.pem"), []byte("first"), 0o600)
	os.WriteFile(filepath.Join(downloads, "a.private-key.pem"), rsaPEM(t), 0o644)
	key, err := PickUpKey(downloads, dest, time.Now().Add(-time.Minute), time.Second, 10*time.Millisecond)
	if err != nil || key != filepath.Join(dest, "a-2.private-key.pem") {
		t.Fatalf("PickUpKey = %q, %v", key, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "a.private-key.pem")); string(b) != "first" {
		t.Fatal("the first key changed")
	}
}
