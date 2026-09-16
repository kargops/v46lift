package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/kargops/v46lift/internal/backend"
	"github.com/kargops/v46lift/internal/config"
	"github.com/kargops/v46lift/internal/network"
)

func Run(ctx context.Context, cfg *config.Config) error {
	netmgr := network.New()
	if err := netmgr.Up(ctx, cfg.Network); err != nil {
		return err
	}
	defer func() {
		_ = netmgr.Down(context.Background())
	}()

	var b backend.Backend
	switch cfg.Engine.Type {
	case "gost":
		b = backend.NewGost(cfg.Engine.Binary, cfg.Mappings)
	default:
		return fmt.Errorf("unsupported backend %q", cfg.Engine.Type)
	}
	return run(ctx, cfg, b)
}

func run(ctx context.Context, cfg *config.Config, b backend.Backend) error {
	if err := b.Start(ctx); err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = b.Stop(stopCtx)
	}()

	cmd := exec.CommandContext(ctx, cfg.Game.Executable, cfg.Game.Args...)
	cmd.Dir = cfg.Game.WorkingDirectory
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start game: %w", err)
	}

	gameDone := make(chan error, 1)
	go func() { gameDone <- cmd.Wait() }()
	backendDone := make(chan error, 1)
	go func() { backendDone <- b.Wait(ctx) }()

	select {
	case err := <-gameDone:
		if err != nil {
			return fmt.Errorf("game exited with error: %w", err)
		}
		return nil
	case err := <-backendDone:
		_ = cmd.Process.Kill()
		<-gameDone
		if err == nil {
			return fmt.Errorf("gost backend exited unexpectedly")
		}
		return fmt.Errorf("gost backend exited: %w", err)
	}
}
