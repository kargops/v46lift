package privilege

import "os/exec"

// Confine runs cmd as the delegating user when this process is root via
// sudo/pkexec. The parent keeps CAP_NET_ADMIN so synthetic addresses can be
// removed after the game exits. The game never inherits those capabilities.
func Confine(cmd *exec.Cmd) {
	confine(cmd)
}

// SetLaunchCaps grants the packed launcher just enough privilege to manage
// synthetic IPv4 addresses without asking the player to run sudo.
func SetLaunchCaps(path string) error {
	return setLaunchCaps(path)
}
