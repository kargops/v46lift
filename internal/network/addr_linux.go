//go:build linux

package network

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func addLoopbackIPv4(ipStr string) error {
	link, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("loopback: %w", err)
	}
	addr, err := netlink.ParseAddr(ipStr + "/32")
	if err != nil {
		return fmt.Errorf("parse %s: %w", ipStr, err)
	}
	addr.Scope = int(netlink.SCOPE_HOST)
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("add %s on lo: %w", ipStr, err)
	}
	return nil
}

func delLoopbackIPv4(ipStr string) error {
	link, err := netlink.LinkByName("lo")
	if err != nil {
		return fmt.Errorf("loopback: %w", err)
	}
	addr, err := netlink.ParseAddr(ipStr + "/32")
	if err != nil {
		return fmt.Errorf("parse %s: %w", ipStr, err)
	}
	if err := netlink.AddrDel(link, addr); err != nil {
		return fmt.Errorf("remove %s from lo: %w", ipStr, err)
	}
	return nil
}

func loopbackHasIPv4(wanted string) (bool, error) {
	iface, err := net.InterfaceByName("lo")
	if err != nil {
		return false, fmt.Errorf("inspect loopback: %w", err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return false, fmt.Errorf("inspect loopback addresses: %w", err)
	}
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			ip = net.ParseIP(a.String())
		}
		if ip != nil && ip.To4() != nil && ip.To4().String() == wanted {
			return true, nil
		}
	}
	return false, nil
}
