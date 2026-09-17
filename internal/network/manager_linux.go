//go:build linux

package network

import (
	"context"

	"github.com/kargops/v46lift/internal/config"
)

type linuxManager struct {
	added []string
}

var (
	loopbackHasIPv4Fn = loopbackHasIPv4
	addLoopbackIPv4Fn = addLoopbackIPv4
	delLoopbackIPv4Fn = delLoopbackIPv4
)

func newPlatformManager() Manager {
	return &linuxManager{}
}

func (m *linuxManager) Up(ctx context.Context, cfg config.NetworkConfig) error {
	if !cfg.ManageSyntheticIPs {
		return nil
	}

	for _, ip := range cfg.SyntheticIPs {
		present, err := loopbackHasIPv4Fn(ip)
		if err != nil {
			_ = m.Down(context.Background())
			return err
		}
		if present {
			continue
		}

		if err := addLoopbackIPv4Fn(ip); err != nil {
			_ = m.Down(context.Background())
			return err
		}
		m.added = append(m.added, ip)
	}
	return nil
}

func (m *linuxManager) Down(ctx context.Context) error {
	var firstErr error
	for i := len(m.added) - 1; i >= 0; i-- {
		ip := m.added[i]
		if err := delLoopbackIPv4Fn(ip); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	m.added = nil
	return firstErr
}
