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
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "validate":
		if err := runValidate(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "print-gost":
		if err := runPrintGost(os.Args[2:]); err != nil {
			fatal(err)
		}
	case "launch":
		if err := runLaunch(os.Args[2:]); err != nil {
			fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `v46lift - legacy IPv4 application compatibility runtime

Usage:
  v46lift validate    --config <file>
  v46lift print-gost  --config <file>
  v46lift launch      --config <file>`)
}

func configFlag(name string, args []string) (*config.Config, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	path := fs.String("config", "", "path to JSON config")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if *path == "" {
		return nil, fmt.Errorf("--config is required")
	}
	return config.Load(*path)
}

func runValidate(args []string) error {
	cfg, err := configFlag("validate", args)
	if err != nil {
		return err
	}
	fmt.Printf("valid: %d mapping(s), engine=%s\n", len(cfg.Mappings), cfg.Engine.Type)
	return nil
}

func runPrintGost(args []string) error {
	cfg, err := configFlag("print-gost", args)
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
	cfg, err := configFlag("launch", args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return lifecycle.Run(ctx, cfg)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "v46lift: %v\n", err)
	os.Exit(1)
}
