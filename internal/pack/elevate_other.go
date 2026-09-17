//go:build !unix

package pack

func alreadyPrivileged() bool {
	return false
}
