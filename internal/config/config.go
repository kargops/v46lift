package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/kargops/v46lift/internal/payload"
)

type Config struct {
	Game     GameConfig    `json:"game"`
	Engine   EngineConfig  `json:"engine"`
	Network  NetworkConfig `json:"network"`
	Mappings []PortMapping `json:"mappings"`
	Pack     PackConfig    `json:"pack,omitempty"`
	Install  *InstallMeta  `json:"install,omitempty"`
}

type GameConfig struct {
	Executable       string   `json:"executable"`
	Args             []string `json:"args,omitempty"`
	WorkingDirectory string   `json:"working_directory,omitempty"`
}

type EngineConfig struct {
	Type   string `json:"type"`
	Binary string `json:"binary,omitempty"`
}

type NetworkConfig struct {
	ManageSyntheticIPs bool     `json:"manage_synthetic_ips"`
	SyntheticIPs       []string `json:"synthetic_ips,omitempty"`
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

// PackConfig is operator-only minting input. It is stripped from baked launchers.
type PackConfig struct {
	Name            string   `json:"name,omitempty"`
	DisplayName     string   `json:"display_name,omitempty"`
	VendorInstaller string   `json:"vendor_installer,omitempty"`
	VendorArgs      []string `json:"vendor_args,omitempty"`
	InstallDir      string   `json:"install_dir,omitempty"`
	WrapPath        string   `json:"wrap_path,omitempty"`
	GostBinary      string   `json:"gost_binary,omitempty"`
}

// InstallMeta is written into baked launchers so wrap/uninstall can stay invisible.
type InstallMeta struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	InstallDir  string `json:"install_dir,omitempty"`
	WrapPath    string `json:"wrap_path,omitempty"`
	RealPath    string `json:"real_path,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return LoadBytes(data, path)
}

func LoadBytes(data []byte, source string) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		if source == "" {
			source = "config"
		}
		return nil, fmt.Errorf("parse %s: %w", source, err)
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

// Resolve finds baked or adjacent configuration without asking the user for a path.
//
// Order: explicit path, V46LIFT_CONFIG, packed launcher payload, config.json beside the binary.
func Resolve(explicitPath, executable string) (*Config, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return Load(explicitPath)
	}
	if env := os.Getenv("V46LIFT_CONFIG"); env != "" {
		return Load(env)
	}
	if executable != "" {
		kind, data, err := payload.Read(executable)
		if err != nil {
			return nil, err
		}
		if kind == payload.KindLauncher {
			return LoadBytes(data, "embedded launcher config")
		}
		sibling := filepath.Join(filepath.Dir(executable), "config.json")
		if st, err := os.Stat(sibling); err == nil && !st.IsDir() {
			return Load(sibling)
		}
	}
	return nil, fmt.Errorf("no configuration found: pass --config, set V46LIFT_CONFIG, or use a packed launcher")
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

func (c *Config) Clone() *Config {
	raw, err := json.Marshal(c)
	if err != nil {
		out := *c
		return &out
	}
	var out Config
	if err := json.Unmarshal(raw, &out); err != nil {
		copied := *c
		return &copied
	}
	return &out
}

func (c *Config) MarshalPretty() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (m PortMapping) Validate() error {
	if m.Protocol != "tcp" && m.Protocol != "udp" {
		return fmt.Errorf("protocol must be tcp or udp")
	}
	ip := net.ParseIP(m.ListenIP)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("listen_ip must be an IPv4 address")
	}
	if m.ListenPort < 1024 || m.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be 1024..65535; GOST runs unprivileged and cannot bind privileged ports")
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
