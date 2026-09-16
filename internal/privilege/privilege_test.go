package privilege

import "testing"

func TestDropIsBestEffort(t *testing.T) {
	if err := Drop(); err != nil {
		t.Fatalf("Drop should not fail for an unprivileged process: %v", err)
	}
}
