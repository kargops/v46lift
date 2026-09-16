package pack

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kargops/v46lift/internal/payload"
)

func Wrap(launcher, wrapPath, realPath string) error {
	if wrapPath == "" || realPath == "" {
		return fmt.Errorf("wrap paths are required")
	}
	if launcher == "" {
		return fmt.Errorf("launcher path is required")
	}

	if st, err := os.Stat(realPath); err == nil && !st.IsDir() {
		info, err := payload.InspectFile(wrapPath)
		if err == nil && info.Kind == payload.KindLauncher {
			return copyFile(launcher, wrapPath, 0755)
		}
		return fmt.Errorf("refusing to wrap %s: %s already exists and %s is not a v46lift launcher", wrapPath, realPath, wrapPath)
	}

	st, err := os.Stat(wrapPath)
	if err != nil {
		return fmt.Errorf("client executable %s not found after install: %w", wrapPath, err)
	}
	if st.IsDir() {
		return fmt.Errorf("client path %s is a directory", wrapPath)
	}

	if err := os.Rename(wrapPath, realPath); err != nil {
		return fmt.Errorf("preserve original client: %w", err)
	}
	if err := copyFile(launcher, wrapPath, 0755); err != nil {
		_ = os.Rename(realPath, wrapPath)
		return fmt.Errorf("install launcher shim: %w", err)
	}
	return nil
}

func Unwrap(wrapPath, realPath string) error {
	if wrapPath == "" || realPath == "" {
		return fmt.Errorf("unwrap paths are required")
	}

	if _, err := os.Stat(realPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("original client %s is missing; not uninstalling the current file at %s", realPath, wrapPath)
		}
		return err
	}

	if st, err := os.Stat(wrapPath); err == nil && !st.IsDir() {
		info, inspectErr := payload.InspectFile(wrapPath)
		if inspectErr == nil && info.Kind == payload.KindLauncher {
			if err := os.Remove(wrapPath); err != nil && !os.IsNotExist(err) {
				if runtime.GOOS == "windows" {
					return fmt.Errorf("could not remove launcher at %s while it is in use; close the game and retry", wrapPath)
				}
				return err
			}
		} else if inspectErr == nil && info.Kind == payload.KindNone {
			return fmt.Errorf("%s does not look like a v46lift launcher; leaving it untouched", wrapPath)
		}
	}

	if _, err := os.Stat(wrapPath); err == nil {
		return fmt.Errorf("cannot restore original client over %s", wrapPath)
	}
	if err := os.Rename(realPath, wrapPath); err != nil {
		return fmt.Errorf("restore original client: %w", err)
	}
	return nil
}
