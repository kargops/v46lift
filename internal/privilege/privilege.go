package privilege

// Drop removes leftover administrative rights after network setup and before
// the unprivileged game process starts.
func Drop() error {
	return drop()
}

// SetLaunchCaps grants the packed launcher just enough privilege to manage
// synthetic IPv4 addresses without asking the player to run sudo.
func SetLaunchCaps(path string) error {
	return setLaunchCaps(path)
}
