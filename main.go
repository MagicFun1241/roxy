package main

import (
	"context"
	"flag"
	"github.com/valyala/fasthttp"
	"github.com/yeqown/fasthttp-reverse-proxy/v2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"sort"
	"strconv"
	"time"
)

func errorHandler(ctx *fasthttp.RequestCtx) {
	ctx.SetStatusCode(400)
	ctx.SetBodyString("Internal error")
}

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

	if config.Security != nil {
		if config.Security.AllowedHosts != nil {
			var t []string
			for _, ip := range config.Security.AllowedHosts {
				if validateIp(ip) {
					t = append(t, ip)
				} else {
					ip, ipNet, err := net.ParseCIDR(ip)
					if err != nil {
						return
					}

					l, _ := AllHosts(ip, ipNet)
					t = append(t, l...)
				}
			}

			sort.Strings(t)
		}
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
			var proxyServer *proxy.ReverseProxy

			server := config.Http.Servers[i]
			if len(server.Upstream) == 1 {
				var upstream = config.Http.Servers[i].Upstream[0]
				if !validateIp(upstream.Host) {
					upstream.Host = LookupIp(ipResolver, upstream.Host)
				}

				proxyServer = proxy.NewReverseProxy(upstream.Host + ":" + strconv.Itoa(int(upstream.Port)))
			} else {
				weights := make(map[string]proxy.Weight)

				for j := 0; j < len(server.Upstream); j++ {
					var upstream = server.Upstream[j]
					if !validateIp(upstream.Host) {
						upstream.Host = LookupIp(ipResolver, upstream.Host)
					}

					if j == 0 && len(weights) == 2 {
						addr := upstream.Host + ":" + strconv.Itoa(int(upstream.Port))
						weights[addr] = proxy.Weight(upstream.Weight - 1)

						assistantProxy := proxy.NewReverseProxy(addr)
						assistantPort := strconv.Itoa(findPort())
						_ = fasthttp.ListenAndServe(":"+assistantPort, func(ctx *fasthttp.RequestCtx) {
							assistantProxy.ServeHTTP(ctx)
						})

						weights[upstream.Host+":"+assistantPort] = proxy.Weight(1)
					} else {
						weights[upstream.Host+":"+strconv.Itoa(int(upstream.Port))] = proxy.Weight(upstream.Weight)
					}
				}

				proxyServer = proxy.NewReverseProxy("", proxy.WithBalancer(weights))
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
							errorHandler(ctx)
							return
						}
					}

					proxyServer.ServeHTTP(ctx)
				}
			}

			if config.Security != nil && config.Security.AllowedHosts != nil {
				handler = func(ctx *fasthttp.RequestCtx) {
					foundIndex := sort.SearchStrings(config.Security.AllowedHosts, ctx.RemoteIP().String())
					if foundIndex == len(config.Security.AllowedHosts) {
						errorHandler(ctx)
						return
					}

					handler(ctx)
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

			var options = make([]proxy.OptionWS, len(server.Upstream))

			for j := 0; j < len(server.Upstream); j++ {
				var upstream = server.Upstream[j]
				options = append(options, proxy.WithURL_OptionWS("ws://"+upstream.Host+":"+strconv.Itoa(int(upstream.Port))))
			}

			proxyServer, _ := proxy.NewWSReverseProxyWith(options...)

			if err := fasthttp.ListenAndServe(":"+strconv.Itoa(int(server.Port)), func(ctx *fasthttp.RequestCtx) {
				proxyServer.ServeHTTP(ctx)
			}); err != nil {
				log.Fatal(err)
			}
		}
	}

	log.Print("Roxy started")
}
