package main

import (
	"context"
	"net"
)

func validateIp(str string) bool {
	return net.ParseIP(str) != nil
}

func LookupIp(resolver *net.Resolver, host string) string {
	ip, _ := resolver.LookupHost(context.Background(), host)
	return ip[0]
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func AllHosts(ip net.IP, ipNet *net.IPNet) ([]string, error) {
	var ips []string
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	lenIPs := len(ips)
	switch {
	case lenIPs < 2:
		return ips, nil

	default:
		return ips[1 : len(ips)-1], nil
	}
}
