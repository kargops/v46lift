package pack

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kargops/v46lift/internal/config"
	"github.com/kargops/v46lift/internal/payload"
	"github.com/kargops/v46lift/internal/privilege"
)

func InstallFromSelf(self string) error {
	info, r, f, err := payload.OpenSection(self)
	if err != nil {
		return err
	}
	defer f.Close()
	if info.Kind != payload.KindInstaller {
		return fmt.Errorf("this binary is not a minted installer")
	}

	zr, err := zip.NewReader(r, info.Length)
	if err != nil {
		return fmt.Errorf("read installer payload: %w", err)
	}

	tmp, err := os.MkdirTemp("", "v46lift-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := extractZip(zr, tmp); err != nil {
		return err
	}

	manifest, err := readManifest(filepath.Join(tmp, "manifest.json"))
	if err != nil {
		return err
	}
	cfg, err := config.Load(filepath.Join(tmp, "config.json"))
	if err != nil {
		return err
	}

	if err := maybeElevate(needsAdmin(manifest)); err != nil {
		return err
	}

	return installExtracted(self, info.Offset, tmp, manifest, cfg)
}

func Uninstall(cfg *config.Config) error {
	if cfg == nil || cfg.Install == nil || cfg.Install.WrapPath == "" || cfg.Install.RealPath == "" {
		return fmt.Errorf("this launcher has no install metadata; nothing to uninstall")
	}
	if err := maybeElevate(needsUninstallAdmin(cfg.Install)); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Restoring original client at %s\n", cfg.Install.WrapPath)
	if err := Unwrap(cfg.Install.WrapPath, cfg.Install.RealPath); err != nil {
		return err
	}
	if cfg.Install.InstallDir != "" {
		fmt.Fprintf(os.Stderr, "Removing %s\n", cfg.Install.InstallDir)
		if err := os.RemoveAll(cfg.Install.InstallDir); err != nil {
			return fmt.Errorf("remove install dir: %w (the client itself has been restored)", err)
		}
	}
	fmt.Fprintln(os.Stderr, "Uninstall complete.")
	return nil
}

func installExtracted(self string, liftLen int64, extracted string, manifest Manifest, cfg *config.Config) error {
	display := firstNonEmpty(manifest.DisplayName, manifest.Name)
	fmt.Fprintf(os.Stderr, "Installing %s compatibility layer...\n", display)

	if err := os.MkdirAll(manifest.InstallDir, 0755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	liftDest := filepath.Join(manifest.InstallDir, liftInstallName())
	if err := writeLauncher(self, liftLen, cfg, liftDest); err != nil {
		return err
	}

	gostSrc := filepath.Join(extracted, firstNonEmpty(manifest.GostRel, gostInstallName()))
	gostDest := filepath.Join(manifest.InstallDir, gostInstallName())
	if err := copyFile(gostSrc, gostDest, 0755); err != nil {
		return fmt.Errorf("install gost: %w", err)
	}

	if err := copyFile(filepath.Join(extracted, "config.json"), filepath.Join(manifest.InstallDir, "config.json"), 0644); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(extracted, "NOTICE-GOST.txt")); err == nil {
		_ = copyFile(filepath.Join(extracted, "NOTICE-GOST.txt"), filepath.Join(manifest.InstallDir, "NOTICE-GOST.txt"), 0644)
	}
	if err := writeUninstallHelper(manifest.InstallDir, liftDest); err != nil {
		return err
	}

	if manifest.VendorRel != "" {
		fmt.Fprintf(os.Stderr, "Installing the client...\n")
		vendor := filepath.Join(extracted, filepath.FromSlash(manifest.VendorRel))
		if runtime.GOOS != "windows" {
			_ = os.Chmod(vendor, 0755)
		}
		cmd := exec.Command(vendor, manifest.VendorArgs...)
		cmd.Dir = filepath.Dir(vendor)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("vendor installer: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "Wrapping %s so the compatibility layer starts automatically...\n", manifest.WrapPath)
	if err := Wrap(liftDest, manifest.WrapPath, RealPath(manifest.WrapPath)); err != nil {
		return err
	}
	if manifest.SetCaps {
		if err := privilege.SetLaunchCaps(manifest.WrapPath); err != nil {
			return fmt.Errorf("make launcher privilege-free at play time: %w", err)
		}
		_ = privilege.SetLaunchCaps(liftDest)
	}

	fmt.Fprintf(os.Stderr, "Done. Start %s as usual; IPv6 access is automatic.\n", display)
	return nil
}

func writeLauncher(self string, liftLen int64, cfg *config.Config, dest string) error {
	raw, err := cfg.MarshalPretty()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	in, err := os.Open(self)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, io.LimitReader(in, liftLen)); err != nil {
		return err
	}
	if _, err := out.Write(raw); err != nil {
		return err
	}
	if err := payload.WriteFooter(out, payload.KindLauncher, uint64(len(raw))); err != nil {
		return err
	}
	return os.Chmod(dest, 0755)
}

func writeUninstallHelper(installDir, liftPath string) error {
	path := filepath.Join(installDir, uninstallName())
	if runtime.GOOS == "windows" {
		body := "@echo off\r\n\"" + liftPath + "\" --v46lift-cli uninstall\r\n"
		return os.WriteFile(path, []byte(body), 0755)
	}
	body := "#!/bin/sh\nexec \"" + liftPath + "\" --v46lift-cli uninstall\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		return err
	}
	return os.Chmod(path, 0755)
}

func readManifest(path string) (Manifest, error) {
	var m Manifest
	raw, err := os.ReadFile(path)
	if err != nil {
		return m, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Name == "" || m.InstallDir == "" || m.WrapPath == "" {
		return m, fmt.Errorf("installer manifest is incomplete")
	}
	return m, nil
}

func extractZip(zr *zip.Reader, dest string) error {
	dest = filepath.Clean(dest)
	for _, f := range zr.File {
		rel := filepath.Clean(filepath.FromSlash(f.Name))
		if rel == "." || rel == string(filepath.Separator) {
			continue
		}
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("invalid installer path %q", f.Name)
		}
		target := filepath.Join(dest, rel)
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("invalid installer path %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		_ = rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if mode := f.Mode(); mode != 0 {
			_ = os.Chmod(target, mode)
		}
	}
	return nil
}

func needsAdmin(m Manifest) bool {
	if m.SetCaps {
		return true
	}
	if !writableLocation(m.InstallDir) {
		return true
	}
	if m.WrapPath != "" && !writableLocation(filepath.Dir(m.WrapPath)) {
		return true
	}
	return false
}

func needsUninstallAdmin(meta *config.InstallMeta) bool {
	if meta.InstallDir != "" && !writableLocation(meta.InstallDir) {
		return true
	}
	if meta.WrapPath != "" && !writableLocation(filepath.Dir(meta.WrapPath)) {
		return true
	}
	return false
}

func writableLocation(path string) bool {
	path = filepath.Clean(path)
	for path != "" {
		st, err := os.Stat(path)
		if err == nil {
			if !st.IsDir() {
				path = filepath.Dir(path)
				continue
			}
			f, err := os.CreateTemp(path, ".v46lift-write-test-*")
			if err != nil {
				return false
			}
			name := f.Name()
			_ = f.Close()
			_ = os.Remove(name)
			return true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return false
		}
		path = parent
	}
	return false
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(dst, mode)
}
