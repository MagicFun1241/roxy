package main

import (
	"context"
	"flag"
	"github.com/valyala/fasthttp"
	reverseProxy "github.com/yeqown/fasthttp-reverse-proxy"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yml", "config file path")
	flag.Parse()

	config := Config{}
	configBytes, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal("Error reading config file")
	}

	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("Error processing config file")
	}

	configBytes = nil

	if config.Dns != nil {
		if !validateIp(*config.Dns) {
			log.Fatal("Invalid DNS")
		}
	} else {
		var dns = "1.1.1.1"
		config.Dns = &dns
	}

	ipResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(5000),
			}
			return d.DialContext(ctx, network, *config.Dns+":53")
		},
	}

	if config.Http != nil {
		for i := 0; i < len(config.Http.Servers); i++ {
			var proxyServer *reverseProxy.ReverseProxy

			server := config.Http.Servers[i]
			if len(server.Upstream) == 1 {
				var upstream = config.Http.Servers[i].Upstream[0]
				if !validateIp(upstream.Host) {
					upstream.Host = lookupIp(ipResolver, upstream.Host)
				}

				proxyServer = reverseProxy.NewReverseProxy(upstream.Host + ":" + strconv.Itoa(int(upstream.Port)))
			} else {
				weights := make(map[string]reverseProxy.Weight)

				for j := 0; j < len(server.Upstream); j++ {
					var upstream = server.Upstream[j]
					if !validateIp(upstream.Host) {
						upstream.Host = lookupIp(ipResolver, upstream.Host)
					}

					if j == 0 && len(weights) == 2 {
						addr := upstream.Host + ":" + strconv.Itoa(int(upstream.Port))
						weights[addr] = reverseProxy.Weight(upstream.Weight - 1)

						assistantProxy := reverseProxy.NewReverseProxy(addr)
						assistantPort := strconv.Itoa(findPort())
						_ = fasthttp.ListenAndServe(":"+assistantPort, func(ctx *fasthttp.RequestCtx) {
							assistantProxy.ServeHTTP(ctx)
						})

						weights[upstream.Host+":"+assistantPort] = reverseProxy.Weight(1)
					} else {
						weights[upstream.Host+":"+strconv.Itoa(int(upstream.Port))] = reverseProxy.Weight(upstream.Weight)
					}
				}

				proxyServer = reverseProxy.NewReverseProxy("", reverseProxy.WithBalancer(weights))
			}

			var handler func(ctx *fasthttp.RequestCtx)

			if server.Domains == nil {
				handler = func(ctx *fasthttp.RequestCtx) {
					proxyServer.ServeHTTP(ctx)
				}
			} else {
				handler = func(ctx *fasthttp.RequestCtx) {
					var host = string(ctx.Host())
					for i, v := range server.Domains {
						if v == host {
							break
						}

						if i == len(server.Domains)-1 {
							return
						}
					}

					proxyServer.ServeHTTP(ctx)
				}
			}

			err := fasthttp.ListenAndServe(":"+strconv.Itoa(int(config.Http.Servers[i].Port)), handler)

			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if config.WebSocket != nil {
		for i := 0; i < len(config.WebSocket.Servers); i++ {
			var server = config.WebSocket.Servers[i]

			options := make([]reverseProxy.OptionWS, len(server.Upstream))

			for j := 0; j < len(server.Upstream); j++ {
				var upstream = server.Upstream[j]
				options = append(options, reverseProxy.WithURL_OptionWS("ws://"+upstream.Host+":"+strconv.Itoa(int(upstream.Port))))
			}

			proxyServer, _ := reverseProxy.NewWSReverseProxyWith(options...)

			if err := fasthttp.ListenAndServe(":"+strconv.Itoa(int(server.Port)), func(ctx *fasthttp.RequestCtx) {
				proxyServer.ServeHTTP(ctx)
			}); err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Print("Roxy started")
}
