package pack

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const realInsert = ".v46lift-real"

func RealPath(wrapPath string) string {
	ext := filepath.Ext(wrapPath)
	return strings.TrimSuffix(wrapPath, ext) + realInsert + ext
}

func DefaultInstallDir(name string) string {
	if runtime.GOOS == "windows" {
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = `C:\Program Files`
		}
		return filepath.Join(pf, "v46lift", name)
	}
	return filepath.Join("/opt/v46lift", name)
}

func gostInstallName() string {
	if runtime.GOOS == "windows" {
		return "gost.exe"
	}
	return "gost"
}

func liftInstallName() string {
	if runtime.GOOS == "windows" {
		return "v46lift.exe"
	}
	return "v46lift"
}

func uninstallName() string {
	if runtime.GOOS == "windows" {
		return "uninstall.cmd"
	}
	return "uninstall"
}

func defaultOutputPath(name string) string {
	base := name + "-setup"
	if runtime.GOOS == "windows" {
		base += ".exe"
	}
	return filepath.Join("dist", base)
}

func ensureExeSuffix(path string) string {
	if runtime.GOOS == "windows" && filepath.Ext(path) == "" {
		return path + ".exe"
	}
	return path
}
