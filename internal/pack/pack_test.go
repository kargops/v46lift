package pack

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kargops/v46lift/internal/config"
	"github.com/kargops/v46lift/internal/payload"
)

func sampleConfig(wrap string) *config.Config {
	return &config.Config{
		Game: config.GameConfig{
			Executable: wrap,
		},
		Engine: config.EngineConfig{
			Type:   "gost",
			Binary: "gost",
		},
		Network: config.NetworkConfig{
			ManageSyntheticIPs: false,
			SyntheticIPs:       []string{"198.18.0.10"},
		},
		Mappings: []config.PortMapping{{
			Protocol:   "tcp",
			ListenIP:   "198.18.0.10",
			ListenPort: 27015,
			TargetHost: "2001:db8:42::10",
			TargetPort: 27015,
		}},
	}
}

func TestRealPath(t *testing.T) {
	if got := RealPath("/opt/game"); got != "/opt/game.v46lift-real" {
		t.Fatalf("got %q", got)
	}
	if got := RealPath(`C:\Games\game.exe`); !strings.HasSuffix(got, "game.v46lift-real.exe") {
		t.Fatalf("got %q", got)
	}
}

func TestWrapAndUnwrap(t *testing.T) {
	dir := t.TempDir()
	wrap := filepath.Join(dir, "game"+exeSuffix())
	real := RealPath(wrap)
	launcher := filepath.Join(dir, "launcher"+exeSuffix())

	if err := os.WriteFile(wrap, []byte("GAME"), 0755); err != nil {
		t.Fatal(err)
	}
	packed := payload.Append([]byte("LIFT"), payload.KindLauncher, []byte(`{"ok":true}`))
	if err := os.WriteFile(launcher, packed, 0755); err != nil {
		t.Fatal(err)
	}

	if err := Wrap(launcher, wrap, real); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(real); err != nil || string(got) != "GAME" {
		t.Fatalf("real=%q err=%v", got, err)
	}
	info, err := payload.InspectFile(wrap)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != payload.KindLauncher {
		t.Fatalf("wrap kind=%s", info.Kind)
	}

	if err := Wrap(launcher, wrap, real); err != nil {
		t.Fatalf("idempotent wrap: %v", err)
	}

	if err := Unwrap(wrap, real); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(wrap); err != nil || string(got) != "GAME" {
		t.Fatalf("restored=%q err=%v", got, err)
	}
	if _, err := os.Stat(real); !os.IsNotExist(err) {
		t.Fatalf("expected real path to be gone, err=%v", err)
	}
}

func TestExtractZipRejectsSlip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: "../evil"}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("nope")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zr, t.TempDir()); err == nil {
		t.Fatal("expected zip slip to be rejected")
	}
}

func TestBuildInstallerContents(t *testing.T) {
	dir := t.TempDir()
	lift := filepath.Join(dir, "v46lift"+exeSuffix())
	gost := filepath.Join(dir, "gost"+exeSuffix())
	vendor := filepath.Join(dir, "vendor-setup"+exeSuffix())
	if err := os.WriteFile(lift, []byte("FAKELIFT"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gost, []byte("FAKEGOST"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vendor, []byte("FAKEVENDOR"), 0755); err != nil {
		t.Fatal(err)
	}

	wrap := filepath.Join(dir, "opt", "game"+exeSuffix())
	out, err := BuildInstaller(Options{
		Config:     sampleConfig(wrap),
		LiftBinary: lift,
		GostBinary: gost,
		Vendor:     vendor,
		Output:     filepath.Join(dir, "setup"+exeSuffix()),
		Name:       "legacy-game",
		InstallDir: filepath.Join(dir, "install"),
		WrapPath:   wrap,
		SetCaps:    BoolPtr(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	info, r, f, err := payload.OpenSection(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if info.Kind != payload.KindInstaller {
		t.Fatalf("kind=%s", info.Kind)
	}
	zr, err := zip.NewReader(r, info.Length)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range zr.File {
		names[file.Name] = true
	}
	for _, want := range []string{"manifest.json", "config.json", gostInstallName(), "NOTICE-GOST.txt", filepath.ToSlash(filepath.Join("vendor", filepath.Base(vendor)))} {
		if !names[want] {
			t.Fatalf("missing %s in %#v", want, names)
		}
	}
}

func TestPackInstallWrapUninstall(t *testing.T) {
	root := moduleRoot(t)
	dir := t.TempDir()

	lift := filepath.Join(dir, "v46lift"+exeSuffix())
	if err := goBuild(t, root, lift, "./cmd/v46lift"); err != nil {
		t.Fatal(err)
	}

	gostSrc := filepath.Join(dir, "fake_gost.go")
	if err := os.WriteFile(gostSrc, []byte(`package main
import "time"
func main() { time.Sleep(time.Hour) }
`), 0644); err != nil {
		t.Fatal(err)
	}
	gost := filepath.Join(dir, "gost"+exeSuffix())
	if err := goBuildFile(t, gost, gostSrc); err != nil {
		t.Fatal(err)
	}

	gameSrc := filepath.Join(dir, "fake_game.go")
	if err := os.WriteFile(gameSrc, []byte(`package main
import "fmt"
func main() { fmt.Println("REAL_GAME") }
`), 0644); err != nil {
		t.Fatal(err)
	}
	gameBin := filepath.Join(dir, "game-payload"+exeSuffix())
	if err := goBuildFile(t, gameBin, gameSrc); err != nil {
		t.Fatal(err)
	}

	wrapDir := filepath.Join(dir, "opt", "legacy-game")
	wrap := filepath.Join(wrapDir, "game"+exeSuffix())
	installDir := filepath.Join(dir, "opt", "v46lift", "legacy-game")

	vendorSrc := filepath.Join(dir, "vendor.go")
	vendorBody := "package main\n" +
		"import (\n\t\"io\"\n\t\"os\"\n)\n" +
		"func main() {\n" +
		"\tin, err := os.Open(`" + gameBin + "`)\n" +
		"\tif err != nil { panic(err) }\n" +
		"\tdefer in.Close()\n" +
		"\tif err := os.MkdirAll(`" + wrapDir + "`, 0755); err != nil { panic(err) }\n" +
		"\tout, err := os.OpenFile(`" + wrap + "`, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)\n" +
		"\tif err != nil { panic(err) }\n" +
		"\tif _, err := io.Copy(out, in); err != nil { panic(err) }\n" +
		"\tif err := out.Close(); err != nil { panic(err) }\n" +
		"}\n"
	if err := os.WriteFile(vendorSrc, []byte(vendorBody), 0644); err != nil {
		t.Fatal(err)
	}
	vendor := filepath.Join(dir, "vendor-setup"+exeSuffix())
	if err := goBuildFile(t, vendor, vendorSrc); err != nil {
		t.Fatal(err)
	}

	setup, err := BuildInstaller(Options{
		Config:      sampleConfig(wrap),
		LiftBinary:  lift,
		GostBinary:  gost,
		Vendor:      vendor,
		Output:      filepath.Join(dir, "legacy-game-setup"+exeSuffix()),
		Name:        "legacy-game",
		DisplayName: "Legacy Game",
		InstallDir:  installDir,
		WrapPath:    wrap,
		SetCaps:     BoolPtr(false),
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(setup)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(installDir, liftInstallName())); err != nil {
		t.Fatalf("lift missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installDir, gostInstallName())); err != nil {
		t.Fatalf("gost missing: %v", err)
	}
	info, err := payload.InspectFile(wrap)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != payload.KindLauncher {
		t.Fatalf("wrapped client kind=%s", info.Kind)
	}
	if got, err := os.ReadFile(RealPath(wrap)); err != nil {
		t.Fatal(err)
	} else if len(got) == 0 {
		t.Fatal("preserved original client is empty")
	}

	un := exec.Command(filepath.Join(installDir, liftInstallName()), "--v46lift-cli", "uninstall")
	unOut, err := un.CombinedOutput()
	if err != nil {
		t.Fatalf("uninstall: %v\n%s", err, unOut)
	}
	if _, err := os.Stat(RealPath(wrap)); !os.IsNotExist(err) {
		t.Fatalf("real client should have been restored over wrap path")
	}
	info, err = payload.InspectFile(wrap)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != payload.KindNone {
		t.Fatalf("restored client still looks packed: %s", info.Kind)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("install dir should have been removed, err=%v", err)
	}
}

func TestPathInside(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "opt", "v46lift", "app")
	inside := filepath.Join(dir, "v46lift")
	outside := filepath.Join(t.TempDir(), "other")
	if !pathInside(inside, dir) {
		t.Fatalf("%s should be inside %s", inside, dir)
	}
	if pathInside(outside, dir) {
		t.Fatalf("%s should not be inside %s", outside, dir)
	}
}

func TestMaybeElevateSkipped(t *testing.T) {
	handedOff, err := maybeElevate(false)
	if handedOff || err != nil {
		t.Fatalf("handedOff=%v err=%v", handedOff, err)
	}
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func goTool() string {
	bin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	return bin
}

func goBuild(t *testing.T, dir, output, pkg string) error {
	t.Helper()
	cmd := exec.Command(goTool(), "build", "-o", output, pkg)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cmdError{err: err, out: out}
	}
	return nil
}

func goBuildFile(t *testing.T, output, src string) error {
	t.Helper()
	cmd := exec.Command(goTool(), "build", "-o", output, src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cmdError{err: err, out: out}
	}
	return nil
}

type cmdError struct {
	err error
	out []byte
}

func (e *cmdError) Error() string {
	return e.err.Error() + "\n" + string(e.out)
}
