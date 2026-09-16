package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

	if err := b.Start(ctx); err != nil {
		return err
	}
	defer func() {
		_ = b.Stop(context.Background())
	}()

	args := append(append([]string{}, cfg.Game.Args...), extraArgs...)
	cmd := exec.CommandContext(ctx, cfg.Game.Executable, args...)
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

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("game exited with error: %w", err)
	}
	return nil
}
