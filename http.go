package main

import (
	"net"
)

func findPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	return port
}
