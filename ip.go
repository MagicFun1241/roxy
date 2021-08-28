package main

import (
	"context"
	"net"
	"time"
)

func validateIp(str string) bool {
	return net.ParseIP(str) != nil
}

func lookupIp(host string, dns string) string {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, dns+":53")
		},
	}
	ip, _ := r.LookupHost(context.Background(), host)

	return ip[0]
}
