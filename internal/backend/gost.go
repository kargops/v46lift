package backend

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/kargops/v46lift/internal/config"
)

type Gost struct {
	binary   string
	mappings []config.PortMapping
	logPath  string

	mu      sync.Mutex
	cmd     *exec.Cmd
	logFile *os.File
}

func NewGost(binary string, mappings []config.PortMapping) *Gost {
	return &Gost{
		binary:   binary,
		mappings: append([]config.PortMapping(nil), mappings...),
	}
}

func (g *Gost) WithLogPath(path string) *Gost {
	g.logPath = path
	return g
}

func (g *Gost) args() []string {
	args := make([]string, 0, len(g.mappings)*2)
	for _, m := range g.mappings {
		listen := net.JoinHostPort(m.ListenIP, fmt.Sprintf("%d", m.ListenPort))
		target := net.JoinHostPort(m.TargetHost, fmt.Sprintf("%d", m.TargetPort))

		service := fmt.Sprintf("%s://%s/%s", m.Protocol, listen, target)
		if m.Protocol == "udp" && (m.UDPKeepalive || m.UDPTTL != "") {
			q := url.Values{}
			if m.UDPKeepalive {
				q.Set("keepalive", "true")
			}
			if m.UDPTTL != "" {
				q.Set("ttl", m.UDPTTL)
			}
			service += "?" + q.Encode()
		}

		args = append(args, "-L", service)
	}
	return args
}

func (g *Gost) CommandLine() string {
	parts := []string{shellishQuote(g.binary)}
	for _, arg := range g.args() {
		parts = append(parts, shellishQuote(arg))
	}
	return strings.Join(parts, " ")
}

func (g *Gost) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cmd != nil {
		return fmt.Errorf("gost backend already started")
	}

	cmd := exec.CommandContext(ctx, g.binary, g.args()...)
	cmd.Stdin = nil
	if g.logPath != "" {
		f, err := os.OpenFile(g.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			cmd.Stdout = f
			cmd.Stderr = f
			g.logFile = f
		} else {
			cmd.Stdout = nil
			cmd.Stderr = nil
		}
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		g.closeLog()
		return fmt.Errorf("start gost: %w", err)
	}
	g.cmd = cmd
	return nil
}

func (g *Gost) Stop(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cmd == nil || g.cmd.Process == nil {
		return nil
	}

	if err := g.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = g.cmd.Process.Kill()
	}
	_, _ = g.cmd.Process.Wait()
	g.cmd = nil
	g.closeLog()
	return nil
}

func (g *Gost) closeLog() {
	if g.logFile != nil {
		_ = g.logFile.Close()
		g.logFile = nil
	}
}

func shellishQuote(s string) string {
	if !strings.ContainsAny(s, " \t\r\n\"'&?") {
		return s
	}
	return fmt.Sprintf("%q", s)
}
