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
		Uid: uid,
		Gid: gid,
	}
}

func delegatedUser() (uint32, uint32, bool) {
	if v := os.Getenv("PKEXEC_UID"); v != "" {
		uid, err := strconv.ParseUint(v, 10, 32)
		if err != nil || uid == 0 {
			return 0, 0, false
		}
		gid := uid
		if u, err := user.LookupId(v); err == nil {
			if g, err := strconv.ParseUint(u.Gid, 10, 32); err == nil {
				gid = g
			}
		}
		return uint32(uid), uint32(gid), true
	}

	uid, uerr := strconv.ParseUint(os.Getenv("SUDO_UID"), 10, 32)
	gid, gerr := strconv.ParseUint(os.Getenv("SUDO_GID"), 10, 32)
	if uerr != nil || gerr != nil || uid == 0 {
		return 0, 0, false
	}
	return uint32(uid), uint32(gid), true
}
