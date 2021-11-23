package main

import (
	"context"
	"net"
)

func validateIp(str string) bool {
	return net.ParseIP(str) != nil
}

func lookupIp(resolver *net.Resolver, host string) string {
	ip, _ := resolver.LookupHost(context.Background(), host)
	return ip[0]
}
