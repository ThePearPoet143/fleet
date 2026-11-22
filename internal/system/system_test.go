package system

import "testing"

func TestDetectKnowsTheMac(t *testing.T) {
	got, err := Detect("darwin")
	if err != nil || got != Mac {
		t.Fatalf("Detect(darwin) = %q, %v; want %q", got, err, Mac)
	}
}

func TestDetectSaysOtherSystemsAreNotSupportedYet(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "freebsd"} {
		_, err := Detect(goos)
		if err == nil || err.Error() != goos+" is not supported yet" {
			t.Errorf("Detect(%s) error = %v; want %q", goos, err, goos+" is not supported yet")
		}
	}
}
