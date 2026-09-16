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

	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error
}

func NewGost(binary string, mappings []config.PortMapping) *Gost {
	return &Gost{
		binary:   binary,
		mappings: append([]config.PortMapping(nil), mappings...),
	}
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gost: %w", err)
	}
	g.cmd = cmd
	g.done = make(chan struct{})
	g.waitErr = nil
	go func() {
		err := cmd.Wait()
		g.mu.Lock()
		g.waitErr = err
		close(g.done)
		g.mu.Unlock()
	}()
	return nil
}

func (g *Gost) Wait(ctx context.Context) error {
	g.mu.Lock()
	done := g.done
	g.mu.Unlock()
	if done == nil {
		return nil
	}

	select {
	case <-done:
		g.mu.Lock()
		err := g.waitErr
		g.mu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Gost) Stop(ctx context.Context) error {
	g.mu.Lock()
	cmd := g.cmd
	done := g.done
	g.mu.Unlock()

	if cmd == nil || cmd.Process == nil || done == nil {
		return nil
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}

	select {
	case <-done:
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		g.clearProcess(cmd)
		return fmt.Errorf("stop gost: %w", ctx.Err())
	}

	g.clearProcess(cmd)
	return nil
}

// clearProcess only clears the process generation stopped by the caller. This
// keeps a delayed, concurrent Stop from clearing a later successful Start.
func (g *Gost) clearProcess(cmd *exec.Cmd) {
	g.mu.Lock()
	if g.cmd == cmd {
		g.cmd = nil
		g.done = nil
	}
	g.mu.Unlock()
}

func shellishQuote(s string) string {
	if !strings.ContainsAny(s, " \t\r\n\"'&?") {
		return s
	}
	return fmt.Sprintf("%q", s)
}
