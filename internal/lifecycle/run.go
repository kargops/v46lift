package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kargops/v46lift/internal/backend"
	"github.com/kargops/v46lift/internal/config"
	"github.com/kargops/v46lift/internal/network"
	"github.com/kargops/v46lift/internal/privilege"
)

func Run(ctx context.Context, cfg *config.Config, extraArgs []string) error {
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
		g := backend.NewGost(cfg.Engine.Binary, cfg.Mappings)
		g.WithCommandSetup(privilege.Confine)
		if cfg.Install != nil && cfg.Install.InstallDir != "" {
			g.WithLogPath(filepath.Join(cfg.Install.InstallDir, "gost.log"))
		}
		b = g
	default:
		return fmt.Errorf("unsupported backend %q", cfg.Engine.Type)
	}
	return run(ctx, cfg, b, extraArgs)
}

func run(ctx context.Context, cfg *config.Config, b backend.Backend, extraArgs []string) error {
	if err := b.Start(ctx); err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), backendStopTimeout)
		defer cancel()
		_ = b.Stop(stopCtx)
	}()

	args := append(append([]string{}, cfg.Game.Args...), extraArgs...)
	cmd := exec.Command(cfg.Game.Executable, args...)
	cmd.Dir = cfg.Game.WorkingDirectory
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	privilege.Confine(cmd)
	if cfg.Install != nil && cfg.Install.WrapPath != "" {
		cmd.Args[0] = cfg.Install.WrapPath
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start game: %w", err)
	}

	gameDone := make(chan error, 1)
	go func() { gameDone <- cmd.Wait() }()
	backendDone := make(chan error, 1)
	go func() { backendDone <- b.Wait(context.Background()) }()

	select {
	case err := <-gameDone:
		if err != nil {
			return fmt.Errorf("game exited with error: %w", err)
		}
		return nil
	case err := <-backendDone:
		killAndWait(cmd.Process, gameDone)
		if err == nil {
			return fmt.Errorf("gost backend exited unexpectedly")
		}
		return fmt.Errorf("gost backend exited: %w", err)
	case <-ctx.Done():
		killAndWait(cmd.Process, gameDone)
		return ctx.Err()
	}
}

const (
	backendStopTimeout = 5 * time.Second
	childKillWait      = time.Second
)

func killAndWait(proc *os.Process, done <-chan error) {
	if proc != nil {
		_ = proc.Kill()
	}
	timer := time.NewTimer(childKillWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}
