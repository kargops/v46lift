package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kargops/v46lift/internal/backend"
	"github.com/kargops/v46lift/internal/config"
	"github.com/kargops/v46lift/internal/lifecycle"
	"github.com/kargops/v46lift/internal/pack"
	"github.com/kargops/v46lift/internal/payload"
)

const cliEscape = "--v46lift-cli"

func main() {
	args := os.Args[1:]
	cliForced := false
	if len(args) > 0 && args[0] == cliEscape {
		cliForced = true
		args = args[1:]
	}

	self, selfErr := os.Executable()
	var kind payload.Kind
	if selfErr == nil && !cliForced {
		info, err := payload.InspectFile(self)
		if err != nil {
			fatal(err)
		}
		kind = info.Kind
	}

	switch {
	case !cliForced && kind == payload.KindInstaller:
		if wantsHelp(args) {
			fmt.Fprintln(os.Stderr, "This installer sets up the application and a hidden IPv6 compatibility layer.")
			fmt.Fprintln(os.Stderr, "Run it as administrator. No configuration files or extra commands are required.")
			return
		}
		if err := pack.InstallFromSelf(self); err != nil {
			fatal(err)
		}
		return
	case !cliForced && kind == payload.KindLauncher:
		cfg, err := config.Resolve("", self)
		if err != nil {
			fatal(err)
		}
		if err := runWithConfig(cfg, args); err != nil {
			fatal(err)
		}
		return
	}

	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "validate":
		if err := runValidate(args[1:]); err != nil {
			fatal(err)
		}
	case "print-gost":
		if err := runPrintGost(args[1:]); err != nil {
			fatal(err)
		}
	case "launch":
		if err := runLaunch(args[1:]); err != nil {
			fatal(err)
		}
	case "pack":
		if err := runPack(args[1:]); err != nil {
			fatal(err)
		}
	case "uninstall":
		if err := runUninstall(args[1:]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `v46lift - legacy IPv4 application compatibility runtime

Mint a seamless installer (operator):
  v46lift pack --config <file> --gost <gost-binary> [--installer <vendor-setup>] [--output <file>]

End users run the minted installer once, then start the application as usual.

Operator commands:
  v46lift validate    [--config <file>]
  v46lift print-gost  [--config <file>]
  v46lift launch      [--config <file>] [game-args...]
  v46lift pack        --config <file> --gost <file> [options]
  v46lift uninstall   [--config <file>]

Packed launchers and installers take no flags. Use --v46lift-cli to reach this UI.`)
}

func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

func resolveConfig(explicit string) (*config.Config, error) {
	self, err := os.Executable()
	if err != nil {
		self = ""
	}
	return config.Resolve(explicit, self)
}

func parseConfigArgs(name string, args []string) (*config.Config, []string, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	path := fs.String("config", "", "path to JSON config (optional for packed launchers)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	cfg, err := resolveConfig(*path)
	if err != nil {
		return nil, nil, err
	}
	return cfg, fs.Args(), nil
}

func runValidate(args []string) error {
	cfg, _, err := parseConfigArgs("validate", args)
	if err != nil {
		return err
	}
	fmt.Printf("valid: %d mapping(s), engine=%s\n", len(cfg.Mappings), cfg.Engine.Type)
	return nil
}

func runPrintGost(args []string) error {
	cfg, _, err := parseConfigArgs("print-gost", args)
	if err != nil {
		return err
	}
	if cfg.Engine.Type != "gost" {
		return fmt.Errorf("print-gost only supports engine.type=gost")
	}
	b := backend.NewGost(cfg.Engine.Binary, cfg.Mappings)
	fmt.Println(b.CommandLine())
	return nil
}

func runLaunch(args []string) error {
	cfg, extra, err := parseConfigArgs("launch", args)
	if err != nil {
		return err
	}
	return runWithConfig(cfg, extra)
}

func runWithConfig(cfg *config.Config, extra []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return lifecycle.Run(ctx, cfg, extra)
}

func runUninstall(args []string) error {
	cfg, _, err := parseConfigArgs("uninstall", args)
	if err != nil {
		return err
	}
	return pack.Uninstall(cfg)
}

func runPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "mapping/config JSON")
	name := fs.String("name", "", "short app name (filesystem-safe)")
	displayName := fs.String("display-name", "", "name shown during install")
	installer := fs.String("installer", "", "vendor client installer to run during setup")
	gost := fs.String("gost", "", "GOST binary to bundle")
	output := fs.String("output", "", "path of the minted installer")
	liftBinary := fs.String("lift-binary", "", "v46lift binary to embed (default: this executable)")
	installDir := fs.String("install-dir", "", "where lift files are installed on the target")
	wrapPath := fs.String("wrap", "", "client executable replaced with the launcher (default: game.executable)")
	noSetCaps := fs.Bool("no-set-caps", false, "do not grant CAP_NET_ADMIN to the launcher")
	gostNotice := fs.String("gost-notice", "", "optional GOST license notice file to bundle")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	opts := pack.Options{
		Config:      cfg,
		LiftBinary:  *liftBinary,
		GostBinary:  *gost,
		Vendor:      *installer,
		Output:      *output,
		Name:        *name,
		DisplayName: *displayName,
		InstallDir:  *installDir,
		WrapPath:    *wrapPath,
	}
	if *noSetCaps {
		opts.SetCaps = pack.BoolPtr(false)
	}
	if *gostNotice != "" {
		data, err := os.ReadFile(*gostNotice)
		if err != nil {
			return err
		}
		opts.GostNotice = string(data)
	}

	out, err := pack.BuildInstaller(opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Minted installer: %s\n", out)
	fmt.Fprintln(os.Stderr, "Give that file to players. They run it once, then start the game as usual.")
	return nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "v46lift: %v\n", err)
	os.Exit(1)
}
