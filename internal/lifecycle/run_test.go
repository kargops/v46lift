package lifecycle

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/kargops/v46lift/internal/config"
)

type exitingBackend struct {
	err     error
	stopped bool
}

func (b *exitingBackend) Start(context.Context) error { return nil }
func (b *exitingBackend) Wait(context.Context) error  { return b.err }
func (b *exitingBackend) Stop(context.Context) error {
	b.stopped = true
	return nil
}

type blockingBackend struct {
	mu      sync.Mutex
	stopCh  chan struct{}
	stopped bool
}

func newBlockingBackend() *blockingBackend {
	return &blockingBackend{stopCh: make(chan struct{})}
}

func (b *blockingBackend) Start(context.Context) error { return nil }

func (b *blockingBackend) Wait(context.Context) error {
	<-b.stopCh
	return nil
}

func (b *blockingBackend) Stop(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.stopped {
		b.stopped = true
		close(b.stopCh)
	}
	return nil
}

func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) && os.Args[i+1] == "lifecycle-sleep" {
			time.Sleep(time.Minute)
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func sleepGameConfig(t *testing.T) *config.Config {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &config.Config{Game: config.GameConfig{
		Executable: executable,
		Args:       []string{"--", "lifecycle-sleep"},
	}}
}

func TestKillAndWaitBoundsWait(t *testing.T) {
	start := time.Now()
	killAndWait(nil, make(chan error))
	if time.Since(start) > childKillWait+200*time.Millisecond {
		t.Fatal("killAndWait did not bound its wait")
	}
}

func TestRunStopsGameWhenBackendExits(t *testing.T) {
	backendErr := errors.New("backend failed")
	b := &exitingBackend{err: backendErr}
	cfg := sleepGameConfig(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- run(ctx, cfg, b, nil) }()

	var err error
	select {
	case err = <-runDone:
	case <-time.After(2 * time.Second):
		cancel()
		<-runDone
		t.Fatal("run did not stop the game after the backend exited")
	}
	if !errors.Is(err, backendErr) {
		t.Fatalf("expected backend error, got %v", err)
	}
	if !b.stopped {
		t.Fatal("backend Stop was not called")
	}
}

func TestRunReturnsContextErrorOnCancel(t *testing.T) {
	b := newBlockingBackend()
	cfg := sleepGameConfig(t)

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- run(ctx, cfg, b, nil) }()
	time.Sleep(100 * time.Millisecond)
	cancel()

	var err error
	select {
	case err = <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return after cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	b.mu.Lock()
	stopped := b.stopped
	b.mu.Unlock()
	if !stopped {
		t.Fatal("backend Stop was not called")
	}
}
