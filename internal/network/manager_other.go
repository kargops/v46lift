//go:build !linux

package network

import (
	"context"
	"fmt"
	"runtime"

	"github.com/kargops/v46lift/internal/config"
)

type unsupportedManager struct{}

func newPlatformManager() Manager {
	return &unsupportedManager{}
}

func (m *unsupportedManager) Up(ctx context.Context, cfg config.NetworkConfig) error {
	if cfg.ManageSyntheticIPs && len(cfg.SyntheticIPs) > 0 {
		return fmt.Errorf("synthetic IPv4 address management is not implemented on %s yet; disable network.manage_synthetic_ips or use an already-local listen address", runtime.GOOS)
	}
	return nil
}

func (m *unsupportedManager) Down(ctx context.Context) error {
	return nil
}
