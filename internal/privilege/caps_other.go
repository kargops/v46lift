//go:build !linux

package privilege

func setLaunchCaps(path string) error {
	return nil
}
