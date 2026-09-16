package config

import "testing"

func TestValidMapping(t *testing.T) {
	m := PortMapping{
		Protocol:   "udp",
		ListenIP:   "198.18.0.10",
		ListenPort: 27015,
		TargetHost: "2001:db8::10",
		TargetPort: 27015,
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid mapping: %v", err)
	}
}

func TestRejectInvalidProtocol(t *testing.T) {
	m := PortMapping{
		Protocol:   "sctp",
		ListenIP:   "198.18.0.10",
		ListenPort: 27015,
		TargetHost: "2001:db8::10",
		TargetPort: 27015,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
