// https://pkg.go.dev/runtime#GOOS
package system

import "fmt"

const Mac = "mac"

func Detect(goos string) (string, error) {
	if goos == "darwin" {
		return Mac, nil
	}
	return "", fmt.Errorf("%s is not supported yet", goos)
}
