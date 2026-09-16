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

func TestGostStopKillsProcessAtDeadline(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	script := "#!/bin/sh\ntrap '' INT\n: > " + ready + "\nwhile :; do sleep 1; done\n"
	g := NewGost(writeExecutable(t, script), nil)
	if err := g.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake gost did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

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
	if err := g.Wait(waitCtx); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected killed child exit status, got %v", err)
		}
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
	deadline = time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = g.cmd.Process.Kill()
			t.Fatal("restarted fake gost did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	restartCtx, restartCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer restartCancel()
	if err := g.Stop(restartCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stop restarted child: expected deadline error, got %v", err)
	}
}
