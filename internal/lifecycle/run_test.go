package lifecycle

import (
	"context"
	"errors"
	"os"
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

func TestMain(m *testing.M) {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) && os.Args[i+1] == "lifecycle-sleep" {
			time.Sleep(time.Minute)
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestRunStopsGameWhenBackendExits(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	backendErr := errors.New("backend failed")
	b := &exitingBackend{err: backendErr}
	cfg := &config.Config{Game: config.GameConfig{
		Executable: executable,
		Args:       []string{"--", "lifecycle-sleep"},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- run(ctx, cfg, b) }()

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
