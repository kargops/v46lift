//go:build linux

package privilege

import (
	"os"
	"os/user"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func drop() error {
	if os.Geteuid() == 0 {
		if uid, gid, ok := delegatedUser(); ok {
			if err := unix.Setgroups([]int{gid}); err != nil {
				return err
			}
			if err := unix.Setgid(gid); err != nil {
				return err
			}
			if err := unix.Setuid(uid); err != nil {
				return err
			}
			return nil
		}
	}
	return clearCapsBestEffort()
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

func clearCapsBestEffort() error {
	header := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData
	err := unix.Capset(&header, &data[0])
	if err == nil || err == syscall.EPERM || err == unix.EPERM {
		return nil
	}
	return err
}
