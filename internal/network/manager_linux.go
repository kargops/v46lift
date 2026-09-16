//go:build linux

package network

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/kargops/v46lift/internal/config"
)

type linuxManager struct {
	added []string
}

func newPlatformManager() Manager {
	return &linuxManager{}
}

func (m *linuxManager) Up(ctx context.Context, cfg config.NetworkConfig) error {
	if !cfg.ManageSyntheticIPs {
		return nil
	}

	for _, ip := range cfg.SyntheticIPs {
		present, err := loopbackHasIPv4(ctx, ip)
		if err != nil {
			_ = m.Down(context.Background())
			return err
		}
		if present {
			continue
		}

		cmd := exec.CommandContext(ctx, "ip", "address", "add", ip+"/32", "dev", "lo")
		out, err := cmd.CombinedOutput()
		if err != nil {
			_ = m.Down(context.Background())
			return fmt.Errorf("add synthetic IP %s: %w: %s", ip, err, strings.TrimSpace(string(out)))
		}
		m.added = append(m.added, ip)
	}
	return nil
}

func (m *linuxManager) Down(ctx context.Context) error {
	var firstErr error
	for i := len(m.added) - 1; i >= 0; i-- {
		ip := m.added[i]
		cmd := exec.CommandContext(ctx, "ip", "address", "delete", ip+"/32", "dev", "lo")
		out, err := cmd.CombinedOutput()
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("remove synthetic IP %s: %w: %s", ip, err, strings.TrimSpace(string(out)))
		}
	}
	m.added = nil
	return firstErr
}

func loopbackHasIPv4(ctx context.Context, wanted string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ip", "-o", "-4", "address", "show", "dev", "lo")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("inspect loopback addresses: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if !strings.Contains(field, "/") {
				continue
			}
			ip, _, err := net.ParseCIDR(field)
			if err == nil && ip.String() == wanted {
				return true, nil
			}
		}
	}
	return false, nil
}
