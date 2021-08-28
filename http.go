package main

import (
	"fmt"
	"github.com/valyala/fasthttp"
	"net"
	"strconv"
)

func startHttpServer(n uint8)  {
	_ = fasthttp.ListenAndServe(":300"+strconv.Itoa(int(n)), func(ctx *fasthttp.RequestCtx) {
		_, _ = fmt.Fprintf(ctx, "Hello from %s", strconv.Itoa(int(n)))
	})
}

func findPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	return port
}
