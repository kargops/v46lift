package pack

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/kargops/v46lift/internal/config"
	"github.com/kargops/v46lift/internal/payload"
)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

type Options struct {
	Config      *config.Config
	LiftBinary  string
	GostBinary  string
	Vendor      string
	VendorArgs  []string
	Output      string
	Name        string
	DisplayName string
	InstallDir  string
	WrapPath    string
	SetCaps     *bool
	GostNotice  string
}

type Manifest struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name,omitempty"`
	InstallDir  string   `json:"install_dir"`
	VendorRel   string   `json:"vendor_installer,omitempty"`
	VendorArgs  []string `json:"vendor_args,omitempty"`
	WrapPath    string   `json:"wrap_path"`
	SetCaps     bool     `json:"set_capabilities"`
	GostRel     string   `json:"gost,omitempty"`
}

func BuildInstaller(opts Options) (string, error) {
	if opts.Config == nil {
		return "", fmt.Errorf("config is required")
	}
	opts.Name = firstNonEmpty(opts.Name, opts.Config.Pack.Name)
	if !nameRe.MatchString(opts.Name) {
		return "", fmt.Errorf("pack name %q must be a short identifier like legacy-game", opts.Name)
	}
	opts.DisplayName = firstNonEmpty(opts.DisplayName, opts.Config.Pack.DisplayName, opts.Name)
	opts.Vendor = firstNonEmpty(opts.Vendor, opts.Config.Pack.VendorInstaller)
	if len(opts.VendorArgs) == 0 {
		opts.VendorArgs = append([]string{}, opts.Config.Pack.VendorArgs...)
	}
	opts.InstallDir = firstNonEmpty(opts.InstallDir, opts.Config.Pack.InstallDir, DefaultInstallDir(opts.Name))
	opts.WrapPath = firstNonEmpty(opts.WrapPath, opts.Config.Pack.WrapPath, opts.Config.Game.Executable)
	opts.GostBinary = firstNonEmpty(opts.GostBinary, opts.Config.Pack.GostBinary)
	if opts.LiftBinary == "" {
		self, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("locate v46lift binary: %w", err)
		}
		opts.LiftBinary = self
	}
	if opts.Output == "" {
		opts.Output = defaultOutputPath(opts.Name)
	}
	opts.Output = ensureExeSuffix(opts.Output)

	if strings.TrimSpace(opts.WrapPath) == "" {
		return "", fmt.Errorf("wrap path is required: set game.executable or --wrap to the binary players actually launch (not the vendor setup program)")
	}
	if opts.GostBinary == "" {
		return "", fmt.Errorf("a GOST binary is required to mint a seamless installer; pass --gost")
	}

	setCaps := runtime.GOOS == "linux" && opts.Config.Network.ManageSyntheticIPs
	if opts.SetCaps != nil {
		setCaps = *opts.SetCaps
	}

	runtimeCfg, err := bakedConfig(opts)
	if err != nil {
		return "", err
	}

	manifest := Manifest{
		Name:        opts.Name,
		DisplayName: opts.DisplayName,
		InstallDir:  opts.InstallDir,
		VendorArgs:  append([]string{}, opts.VendorArgs...),
		WrapPath:    opts.WrapPath,
		SetCaps:     setCaps,
		GostRel:     gostInstallName(),
	}
	if opts.Vendor != "" {
		manifest.VendorRel = filepath.ToSlash(filepath.Join("vendor", filepath.Base(opts.Vendor)))
	}

	tmp, err := os.CreateTemp("", "v46lift-pack-*.zip")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	zw := zip.NewWriter(tmp)
	if err := writeZipContents(zw, opts, manifest, runtimeCfg); err != nil {
		_ = zw.Close()
		_ = tmp.Close()
		return "", err
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		_ = tmp.Close()
		return "", err
	}
	st, err := tmp.Stat()
	if err != nil {
		_ = tmp.Close()
		return "", err
	}

	lift, err := os.ReadFile(opts.LiftBinary)
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("read lift binary: %w", err)
	}
	lift = payload.Strip(lift)

	if err := os.MkdirAll(filepath.Dir(opts.Output), 0755); err != nil {
		_ = tmp.Close()
		return "", err
	}
	out, err := os.OpenFile(opts.Output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	defer out.Close()

	if _, err := out.Write(lift); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if _, err := io.Copy(out, tmp); err != nil {
		_ = tmp.Close()
		return "", err
	}
	_ = tmp.Close()
	if err := payload.WriteFooter(out, payload.KindInstaller, uint64(st.Size())); err != nil {
		return "", err
	}
	if err := os.Chmod(opts.Output, 0755); err != nil {
		return "", err
	}
	return opts.Output, nil
}

func bakedConfig(opts Options) (*config.Config, error) {
	cfg := opts.Config.Clone()
	cfg.Pack = config.PackConfig{}
	cfg.Game.Executable = RealPath(opts.WrapPath)
	if cfg.Game.WorkingDirectory == "" {
		cfg.Game.WorkingDirectory = filepath.Dir(opts.WrapPath)
	}
	cfg.Engine.Type = "gost"
	cfg.Engine.Binary = filepath.Join(opts.InstallDir, gostInstallName())
	cfg.Install = &config.InstallMeta{
		Name:        opts.Name,
		DisplayName: opts.DisplayName,
		InstallDir:  opts.InstallDir,
		WrapPath:    opts.WrapPath,
		RealPath:    RealPath(opts.WrapPath),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeZipContents(zw *zip.Writer, opts Options, manifest Manifest, runtimeCfg *config.Config) error {
	manJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	cfgJSON, err := runtimeCfg.MarshalPretty()
	if err != nil {
		return err
	}
	if err := addBytes(zw, "manifest.json", manJSON, 0644); err != nil {
		return err
	}
	if err := addBytes(zw, "config.json", cfgJSON, 0644); err != nil {
		return err
	}
	if err := addFile(zw, gostInstallName(), opts.GostBinary, 0755); err != nil {
		return fmt.Errorf("bundle gost: %w", err)
	}
	notice := opts.GostNotice
	if notice == "" {
		notice = defaultGostNotice()
	}
	if err := addBytes(zw, "NOTICE-GOST.txt", []byte(notice), 0644); err != nil {
		return err
	}
	if opts.Vendor != "" {
		if err := addFile(zw, manifest.VendorRel, opts.Vendor, 0755); err != nil {
			return fmt.Errorf("bundle vendor installer: %w", err)
		}
	}
	return nil
}

func addBytes(zw *zip.Writer, name string, data []byte, mode os.FileMode) error {
	hdr := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	hdr.SetMode(mode)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func addFile(zw *zip.Writer, name, src string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("%s is a directory", src)
	}
	hdr, err := zip.FileInfoHeader(st)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Deflate
	hdr.SetMode(mode)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func defaultGostNotice() string {
	return "This installer bundles a GOST binary (https://github.com/go-gost/gost, https://v3.gost.run/).\n" +
		"GOST is a separate project with its own license and copyright holders.\n" +
		"v46lift is MIT-licensed and is not affiliated with GOST.\n"
}

func BoolPtr(v bool) *bool { return &v }
