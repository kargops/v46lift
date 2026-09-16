package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Game     GameConfig    `json:"game"`
	Engine   EngineConfig  `json:"engine"`
	Network  NetworkConfig `json:"network"`
	Mappings []PortMapping `json:"mappings"`
}

type GameConfig struct {
	Executable       string   `json:"executable"`
	Args             []string `json:"args"`
	WorkingDirectory string   `json:"working_directory"`
}

type EngineConfig struct {
	Type   string `json:"type"`
	Binary string `json:"binary"`
}

type NetworkConfig struct {
	ManageSyntheticIPs bool     `json:"manage_synthetic_ips"`
	SyntheticIPs       []string `json:"synthetic_ips"`
}

type PortMapping struct {
	Protocol     string `json:"protocol"`
	ListenIP     string `json:"listen_ip"`
	ListenPort   int    `json:"listen_port"`
	TargetHost   string `json:"target_host"`
	TargetPort   int    `json:"target_port"`
	UDPKeepalive bool   `json:"udp_keepalive,omitempty"`
	UDPTTL       string `json:"udp_ttl,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Game.WorkingDirectory == "" && cfg.Game.Executable != "" {
		cfg.Game.WorkingDirectory = filepath.Dir(cfg.Game.Executable)
	}
	if cfg.Engine.Type == "" {
		cfg.Engine.Type = "gost"
	}
	if cfg.Engine.Binary == "" {
		cfg.Engine.Binary = "gost"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Game.Executable) == "" {
		return fmt.Errorf("game.executable is required")
	}

	switch c.Engine.Type {
	case "gost":
	default:
		return fmt.Errorf("unsupported engine.type %q", c.Engine.Type)
	}

	if len(c.Mappings) == 0 {
		return fmt.Errorf("at least one mapping is required")
	}

	for i, m := range c.Mappings {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("mapping %d: %w", i, err)
		}
	}

	for i, ip := range c.Network.SyntheticIPs {
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			return fmt.Errorf("network.synthetic_ips[%d] must be an IPv4 address", i)
		}
	}

	return nil
}

func (m PortMapping) Validate() error {
	if m.Protocol != "tcp" && m.Protocol != "udp" {
		return fmt.Errorf("protocol must be tcp or udp")
	}
	ip := net.ParseIP(m.ListenIP)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("listen_ip must be an IPv4 address")
	}
	if m.ListenPort < 1 || m.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be 1..65535")
	}
	if m.TargetHost == "" {
		return fmt.Errorf("target_host is required")
	}
	if m.TargetPort < 1 || m.TargetPort > 65535 {
		return fmt.Errorf("target_port must be 1..65535")
	}
	if m.UDPTTL != "" && m.Protocol != "udp" {
		return fmt.Errorf("udp_ttl is only valid for udp mappings")
	}
	return nil
}
