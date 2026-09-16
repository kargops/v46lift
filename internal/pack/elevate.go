package pack

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func maybeElevate(need bool) (handedOff bool, err error) {
	if !need || alreadyPrivileged() || os.Getenv("V46LIFT_ELEVATED") == "1" {
		return false, nil
	}
	return true, reexecElevated()
}

func reexecElevated() error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate installer: %w", err)
	}
	self, err = filepath.Abs(self)
	if err != nil {
		return err
	}

	env := append(os.Environ(), "V46LIFT_ELEVATED=1")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = windowsRunAs(self, os.Args[1:])
	} else {
		cmd = unixElevate(self, os.Args[1:])
	}
	if cmd == nil {
		return fmt.Errorf("administrator privileges are required; rerun this installer as administrator")
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Run()
}

func unixElevate(self string, args []string) *exec.Cmd {
	for _, helper := range []string{"pkexec", "sudo"} {
		path, err := exec.LookPath(helper)
		if err != nil {
			continue
		}
		return exec.Command(path, append([]string{self}, args...)...)
	}
	return nil
}

func windowsRunAs(self string, args []string) *exec.Cmd {
	ps, err := exec.LookPath("powershell")
	if err != nil {
		ps = "powershell"
	}
	argList := ""
	if len(args) > 0 {
		quoted := make([]string, 0, len(args))
		for _, a := range args {
			quoted = append(quoted, "'"+strings.ReplaceAll(a, "'", "''")+"'")
		}
		argList = " -ArgumentList " + strings.Join(quoted, ",")
	}
	script := fmt.Sprintf(
		"Start-Process -FilePath '%s'%s -Verb RunAs -Wait",
		strings.ReplaceAll(self, "'", "''"),
		argList,
	)
	return exec.Command(ps, "-NoProfile", "-Command", script)
}
