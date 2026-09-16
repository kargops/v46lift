//go:build linux

package privilege

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func confine(cmd *exec.Cmd) {
	if cmd == nil || os.Geteuid() != 0 {
		return
	}
	uid, gid, ok := delegatedUser()
	if !ok {
		return
	}
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
		cmd.SysProcAttr = attr
	}
	attr.Credential = &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}
}

func delegatedUser() (int, int, bool) {
	if v := os.Getenv("PKEXEC_UID"); v != "" {
		uid, err := strconv.Atoi(v)
		if err != nil || uid == 0 {
			return 0, 0, false
		}
		gid := uid
		if u, err := user.LookupId(v); err == nil {
			if g, err := strconv.Atoi(u.Gid); err == nil {
				gid = g
			}
		}
		return uid, gid, true
	}

	uid, uerr := strconv.Atoi(os.Getenv("SUDO_UID"))
	gid, gerr := strconv.Atoi(os.Getenv("SUDO_GID"))
	if uerr != nil || gerr != nil || uid == 0 {
		return 0, 0, false
	}
	return uid, gid, true
}
