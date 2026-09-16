//go:build unix

package pack

import "os"

func alreadyPrivileged() bool {
	return os.Geteuid() == 0
}
