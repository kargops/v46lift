package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kargops/v46lift/internal/payload"
)

func TestValidMapping(t *testing.T) {
	m := PortMapping{
		Protocol:   "udp",
		ListenIP:   "198.18.0.10",
		ListenPort: 27015,
		TargetHost: "2001:db8::10",
		TargetPort: 27015,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid mapping: %v", err)
	}
}

func TestRejectPrivilegedListenPort(t *testing.T) {
	m := PortMapping{
		Protocol:   "tcp",
		ListenIP:   "198.18.0.10",
		ListenPort: 80,
		TargetHost: "2001:db8::10",
		TargetPort: 80,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected privileged listen port to be rejected")
	}
}

func TestRejectInvalidProtocol(t *testing.T) {
	m := PortMapping{
		Protocol:   "sctp",
		ListenIP:   "198.18.0.10",
		ListenPort: 27015,
		TargetHost: "2001:db8::10",
		TargetPort: 27015,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadBytesAndResolveEmbedded(t *testing.T) {
	raw := []byte(`{
		"game": {"executable": "/opt/legacy-game/game"},
		"mappings": [{
			"protocol": "tcp",
			"listen_ip": "198.18.0.10",
			"listen_port": 27015,
			"target_host": "2001:db8::10",
			"target_port": 27015
		}]
	}`)
	packed := payload.Append([]byte("lift"), payload.KindLauncher, raw)
	path := filepath.Join(t.TempDir(), "launcher")
	if err := os.WriteFile(path, packed, 0755); err != nil {
		t.Fatal(err)
	}

	cfg, err := Resolve("", path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Engine.Type != "gost" || cfg.Game.Executable != "/opt/legacy-game/game" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Game.WorkingDirectory != filepath.Dir(cfg.Game.Executable) {
		t.Fatalf("working directory=%q", cfg.Game.WorkingDirectory)
	}
}

func TestResolveSiblingConfig(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "v46lift")
	if err := os.WriteFile(exe, []byte("bin"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "config.json")
	raw := []byte(`{
		"game": {"executable": "/opt/legacy-game/game"},
		"mappings": [{
			"protocol": "udp",
			"listen_ip": "198.18.0.10",
			"listen_port": 27015,
			"target_host": "2001:db8::10",
			"target_port": 27015
		}]
	}`)
	if err := os.WriteFile(cfgPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Resolve("", exe)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mappings[0].Protocol != "udp" {
		t.Fatalf("protocol=%s", cfg.Mappings[0].Protocol)
	}
}
