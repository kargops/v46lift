//go:build !linux

package privilege

func drop() error {
	return nil
}
