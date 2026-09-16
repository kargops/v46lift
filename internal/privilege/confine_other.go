//go:build !linux

package privilege

import "os/exec"

func confine(cmd *exec.Cmd) {}
