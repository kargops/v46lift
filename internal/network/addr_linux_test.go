//go:build linux

package network

import "testing"

func TestLoopbackHasLocalhost(t *testing.T) {
	ok, err := loopbackHasIPv4("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected 127.0.0.1 on loopback")
	}
}
