//go:build linux

package privilege

import (
	"encoding/binary"
	"fmt"
	"os/exec"

	"golang.org/x/sys/unix"
)

const (
	vfsCapRevision2     = 0x02000000
	vfsCapFlagEffective = 0x00000001
)

func setLaunchCaps(path string) error {
	if err := setCapabilityXattr(path, unix.CAP_NET_ADMIN, unix.CAP_NET_BIND_SERVICE); err == nil {
		return nil
	} else if setcap, lookErr := exec.LookPath("setcap"); lookErr == nil {
		cmd := exec.Command(setcap, "cap_net_admin,cap_net_bind_service=ep", path)
		if out, runErr := cmd.CombinedOutput(); runErr != nil {
			return fmt.Errorf("grant launch capabilities: %w: %s", runErr, string(out))
		}
		return nil
	} else {
		return fmt.Errorf("grant launch capabilities: %w", err)
	}
}

func setCapabilityXattr(path string, caps ...int) error {
	var permitted [2]uint32
	for _, c := range caps {
		if c < 0 {
			continue
		}
		permitted[c>>5] |= 1 << (uint(c) & 31)
	}

	buf := make([]byte, 20)
	binary.LittleEndian.PutUint32(buf[0:4], vfsCapRevision2|vfsCapFlagEffective)
	binary.LittleEndian.PutUint32(buf[4:8], permitted[0])
	binary.LittleEndian.PutUint32(buf[8:12], 0)
	binary.LittleEndian.PutUint32(buf[12:16], permitted[1])
	binary.LittleEndian.PutUint32(buf[16:20], 0)
	return unix.Setxattr(path, "security.capability", buf, 0)
}
