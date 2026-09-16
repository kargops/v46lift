package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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

	if err := b.Start(ctx); err != nil {
		return err
	}
	defer func() {
		_ = b.Stop(context.Background())
	}()

	cmd := exec.CommandContext(ctx, cfg.Game.Executable, cfg.Game.Args...)
	cmd.Dir = cfg.Game.WorkingDirectory
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start game: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("game exited with error: %w", err)
	}
	return nil
}
