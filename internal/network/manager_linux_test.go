//go:build linux

package network

import (
	"context"
	"errors"
	"testing"

	"github.com/kargops/v46lift/internal/config"
)

func TestUpRollsBackAddressesWhenInspectionFails(t *testing.T) {
	oldProbe := loopbackHasIPv4Fn
	oldAdd := addLoopbackIPv4Fn
	oldDel := delLoopbackIPv4Fn
	t.Cleanup(func() {
		loopbackHasIPv4Fn = oldProbe
		addLoopbackIPv4Fn = oldAdd
		delLoopbackIPv4Fn = oldDel
	})

	probeCalls := 0
	calls := []string{}
	loopbackHasIPv4Fn = func(ip string) (bool, error) {
		probeCalls++
		if probeCalls == 2 {
			return false, errors.New("inspection failed")
		}
		return false, nil
	}
	addLoopbackIPv4Fn = func(ip string) error {
		calls = append(calls, "add "+ip)
		return nil
	}
	delLoopbackIPv4Fn = func(ip string) error {
		calls = append(calls, "del "+ip)
		return nil
	}

	m := &linuxManager{}
	err := m.Up(context.Background(), config.NetworkConfig{
		ManageSyntheticIPs: true,
		SyntheticIPs:       []string{"198.18.0.10", "198.18.0.11"},
	})
	if err == nil {
		t.Fatal("expected second address inspection to fail")
	}
	if len(calls) != 2 || calls[0] != "add 198.18.0.10" || calls[1] != "del 198.18.0.10" {
		t.Fatalf("unexpected operation order: %v", calls)
	}
	if len(m.added) != 0 {
		t.Fatalf("expected rollback to clear tracked addresses, got %v", m.added)
	}
}
