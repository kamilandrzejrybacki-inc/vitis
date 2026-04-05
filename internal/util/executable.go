package util

import "os/exec"

func LookPath(name string) (string, error) {
	return exec.LookPath(name)
}
