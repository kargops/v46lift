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
	"time"

	"github.com/kargops/v46lift/internal/config"
)

// forceKillWait is how long Stop waits for cmd.Wait after Kill. If the child
// is stuck (for example in uninterruptible kernel I/O), Stop returns anyway
// so callers can tear down the rest of the runtime.
const forceKillWait = time.Second

type Gost struct {
	binary   string
	mappings []config.PortMapping
	logPath  string
	setupCmd func(*exec.Cmd)

	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
	waitErr error
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

func (g *Gost) WithCommandSetup(fn func(*exec.Cmd)) *Gost {
	g.setupCmd = fn
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
	if err := ctx.Err(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cmd != nil {
		return fmt.Errorf("gost backend already started")
	}

	// Command, not CommandContext: Stop owns shutdown. Binding the child to
	// the parent context would SIGKILL it on cancel and race graceful stop.
	cmd := exec.Command(g.binary, g.args()...)
	cmd.Stdin = nil
	if g.setupCmd != nil {
		g.setupCmd(cmd)
	}
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
		g.closeLogLocked()
		return fmt.Errorf("start gost: %w", err)
	}
	g.cmd = cmd
	done := make(chan struct{})
	g.done = done
	g.waitErr = nil
	go func() {
		err := cmd.Wait()
		g.mu.Lock()
		if g.cmd == cmd || g.cmd == nil {
			g.waitErr = err
		}
		close(done)
		g.mu.Unlock()
	}()
	return nil
}

func (g *Gost) Wait(ctx context.Context) error {
	g.mu.Lock()
	done := g.done
	waitErr := g.waitErr
	g.mu.Unlock()
	if done == nil {
		return waitErr
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
		g.clearProcess(cmd)
		return nil
	case <-ctx.Done():
		_ = cmd.Process.Kill()
	}

	waitDone(done, forceKillWait)
	g.clearProcess(cmd)
	return fmt.Errorf("stop gost: %w", ctx.Err())
}

func waitDone(done <-chan struct{}, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
}

// clearProcess only clears the process generation stopped by the caller. This
// keeps a delayed, concurrent Stop from clearing a later successful Start.
func (g *Gost) clearProcess(cmd *exec.Cmd) {
	g.mu.Lock()
	if g.cmd == cmd {
		g.cmd = nil
		g.done = nil
		g.closeLogLocked()
	}
	g.mu.Unlock()
}

func (g *Gost) closeLogLocked() {
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
