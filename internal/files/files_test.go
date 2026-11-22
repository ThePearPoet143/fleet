package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesTheFileWithTheModeGiven(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "service.env")
	for _, text := range []string{"old\n", "new\n"} {
		if err := Write(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(got) != "new\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("%q %v", got, info.Mode())
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}
