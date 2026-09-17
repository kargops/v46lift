//go:build !windows

package backend

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func writeExecutable(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-gost")
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func loopingScript(ready string) string {
	return "#!/bin/sh\n: > " + ready + "\nwhile :; do sleep 1; done\n"
}

func waitReady(t *testing.T, g *Gost, ready string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			return
		}
		if time.Now().After(deadline) {
			if g.cmd != nil && g.cmd.Process != nil {
				_ = g.cmd.Process.Kill()
			}
			t.Fatal("fake gost did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWaitDoneReturnsOnTimeout(t *testing.T) {
	done := make(chan struct{})
	start := time.Now()
	waitDone(done, 20*time.Millisecond)
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("waitDone did not bound its wait")
	}
}

func TestGostWaitReportsUnexpectedExit(t *testing.T) {
	g := NewGost(writeExecutable(t, "#!/bin/sh\nexit 17\n"), nil)
	if err := g.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	err := g.Wait(context.Background())
	var exitErr *exec.ExitError
	if err == nil {
		t.Fatal("expected process exit error")
	}
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
		t.Fatalf("expected exit status 17, got %v", err)
	}
}

func TestGostStartRejectedWhenContextAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := NewGost(writeExecutable(t, "#!/bin/sh\nexit 0\n"), nil)
	if err := g.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGostSurvivesStartContextCancel(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	g := NewGost(writeExecutable(t, loopingScript(ready)), nil)
	ctx, cancel := context.WithCancel(context.Background())
	if err := g.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitReady(t, g, ready)
	cancel()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer waitCancel()
	if err := g.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("gost should keep running after Start context cancel, got %v", err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := g.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestGostStopKillsProcessAtDeadline(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	script := "#!/bin/sh\ntrap '' INT\n: > " + ready + "\nwhile :; do sleep 1; done\n"
	g := NewGost(writeExecutable(t, script), nil)
	if err := g.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitReady(t, g, ready)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- g.Stop(ctx) }()
	var stopErr error
	select {
	case stopErr = <-stopDone:
	case <-time.After(500 * time.Millisecond):
		_ = g.cmd.Process.Kill()
		<-stopDone
		t.Fatal("Stop did not honor its context deadline")
	}
	if !errors.Is(stopErr, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", stopErr)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	err := g.Wait(waitCtx)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected killed child exit status, got %v", err)
	}

	// A timed-out Stop still reaps the old child and must leave the backend
	// reusable. In particular, retaining cmd here would make Start report that
	// the backend was already started.
	if err := os.Remove(ready); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := g.Start(context.Background()); err != nil {
		t.Fatalf("restart after forced stop: %v", err)
	}
	waitReady(t, g, ready)
	restartCtx, restartCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer restartCancel()
	if err := g.Stop(restartCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop restarted child: expected deadline error, got %v", err)
	}
}
