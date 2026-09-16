package backend

import (
	"strings"
	"testing"

	"github.com/kargops/v46lift/internal/config"
)

func TestGostCommandLineIPv6(t *testing.T) {
	g := NewGost("gost", []config.PortMapping{
		{
			Protocol:     "udp",
			ListenIP:     "198.18.0.10",
			ListenPort:   27015,
			TargetHost:   "2001:db8::10",
			TargetPort:   27016,
			UDPKeepalive: true,
			UDPTTL:       "2m",
		},
	})
	line := g.CommandLine()

	for _, expected := range []string{
		"udp://198.18.0.10:27015/[2001:db8::10]:27016",
		"keepalive=true",
		"ttl=2m",
	} {
		if !strings.Contains(line, expected) {
			t.Fatalf("expected %q in %q", expected, line)
		}
	}
}
