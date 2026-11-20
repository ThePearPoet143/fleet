// https://pkg.go.dev/os/exec#LookPath
package setup

import (
	"os"
	"os/exec"

	"github.com/ThePearPoet143/fleet/internal/doctor"
)

func on(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func Detect() Detected {
	var d Detected
	for _, t := range Tools {
		if on(t.Key) {
			d.Tools = append(d.Tools, t.Key)
		}
	}
	if _, err := os.Stat(doctor.TailscaleApp); err == nil || on("tailscale") {
		d.Reach = append(d.Reach, "tailscale")
	}
	if on("t3") {
		d.Reach = append(d.Reach, "t3")
	}
	return d
}
