//go:build linux

package network

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kargops/v46lift/internal/config"
)

func TestUpRollsBackAddressesWhenInspectionFails(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls")
	countPath := filepath.Join(dir, "count")
	script := `#!/bin/sh
case "$*" in
  "-o -4 address show dev lo")
    count=0
    test ! -f "$IP_COUNT" || count=$(cat "$IP_COUNT")
    count=$((count + 1))
    echo "$count" > "$IP_COUNT"
    test "$count" -lt 2
    ;;
  address\ add*|address\ delete*)
    echo "$*" >> "$IP_LOG"
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "ip"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("IP_LOG", logPath)
	t.Setenv("IP_COUNT", countPath)

	m := &linuxManager{}
	err := m.Up(context.Background(), config.NetworkConfig{
		ManageSyntheticIPs: true,
		SyntheticIPs:       []string{"198.18.0.10", "198.18.0.11"},
	})
	if err == nil {
		t.Fatal("expected second address inspection to fail")
	}

	calls, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	got := string(calls)
	for _, want := range []string{
		"address add 198.18.0.10/32 dev lo",
		"address delete 198.18.0.10/32 dev lo",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in calls:\n%s", want, got)
		}
	}
}
