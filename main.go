package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/MagicFun1241/fasthttp-reverse-proxy/v2"
	"github.com/fasthttp/router"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/valyala/fasthttp"
	"github.com/yeqown/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yml", "config file path")
	flag.Parse()

	proxy.SetProduction()

	config := Config{}
	configBytes, err := ioutil.ReadFile(*configPath)
	if err != nil {
		Fatal("Error reading config file")
		return
	}

	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		Fatal("Error processing config file")
		return
	}

	configBytes = nil

	if config.Dns != nil {
		if !validateIp(*config.Dns) {
			Fatal("Invalid DNS")
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

	if config.Security != nil {
		if config.Security.AllowedHostsGroups != nil {
			for groupName, groupValues := range config.Security.AllowedHostsGroups {
				var t []string
				for _, ip := range groupValues {
					if validateIp(ip) {
						t = append(t, ip)
					} else {
						ip, ipNet, err := net.ParseCIDR(ip)
						if err != nil {
							Fatalf("Invalid ip or mask passed for '%s' allowedHostsGroup\n", groupName)
							return
						}

						l, _ := AllHosts(ip, ipNet)
						t = append(t, l...)
					}
				}

				sort.Strings(t)
				config.Security.AllowedHostsGroups[groupName] = t
			}
		}
	}

	if config.Http != nil {
		for i := 0; i < len(config.Http.Servers); i++ {
			var proxyServer *proxy.ReverseProxy
			var middlewares []func(ctx *fasthttp.RequestCtx)

			server := config.Http.Servers[i]

			if server.AllowedHostsGroups != nil {
				for _, group := range server.AllowedHostsGroups {
					if config.Security.AllowedHostsGroups[group] == nil {
						Fatalf("Item refers to not exist group '%s'\n", group)
						return
					}
				}
			}

			if len(server.Upstream) == 1 {
				var upstream = config.Http.Servers[i].Upstream[0]
				if !validateIp(upstream.Host) {
					upstream.Host = LookupIp(ipResolver, upstream.Host)
				}

				if server.Quic != nil || *server.Quic {
					if server.QuicKey == nil || server.QuicCertificate == nil {
						log.Error("'quicKey' and 'quicCertificate' must be passed")
						return
					}

					if _, err := os.Stat(*server.QuicKey); os.IsNotExist(err) {
						log.Error("http3 key file not found")
						return
					}

					if _, err := os.Stat(*server.QuicCertificate); os.IsNotExist(err) {
						log.Error("http3 certificate file not found")
						return
					}

					go func() {
						originUrl, _ := url.Parse(fmt.Sprintf("http://%s:%d", server.Upstream[0].Host, server.Upstream[0].Port))
						addr := fmt.Sprintf(":%d", server.Port)
						Infof("Starting http3 server at %s", addr)

						httpProxy := httputil.NewSingleHostReverseProxy(originUrl)
						err = http3.ListenAndServeQUIC(addr, *server.QuicCertificate, *server.QuicKey, httpProxy)
						if err != nil {
							Fatal(err)
						}
					}()
				}

				proxyServer = proxy.NewReverseProxy(upstream.Host + ":" + strconv.Itoa(int(upstream.Port)))
			} else {
				weights := make(map[string]proxy.Weight)

				if server.Quic != nil && *server.Quic {
					log.Warn("http3 load balancing is not supported")
				}

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

			r := router.New()

			if server.Domains != nil {
				middlewares = append(middlewares, func(ctx *fasthttp.RequestCtx) {
					var host = string(ctx.Host())
					for i, v := range server.Domains {
						if v == host {
							break
						}

						if i == len(server.Domains)-1 {
							ctx.Error("Unknown domain", fasthttp.StatusForbidden)
						}
					}
				})
			}

			if config.Security != nil && config.Security.AllowedHostsGroups != nil && server.AllowedHostsGroups != nil {
				middlewares = append(middlewares, func(ctx *fasthttp.RequestCtx) {
					for i, group := range server.AllowedHostsGroups {
						ips := config.Security.AllowedHostsGroups[group]
						foundIndex := sort.SearchStrings(ips, ctx.RemoteIP().String())

						if foundIndex == len(ips) && i == len(server.AllowedHostsGroups)-1 {
							Errorf("Connection from %s to :%d is not allowed with groups %v", ctx.RemoteIP().String(), server.Port, server.AllowedHostsGroups)
							ctx.Error("Access denied", fasthttp.StatusForbidden)
						}
					}
				})
			}

			r.ANY("/*", func(ctx *fasthttp.RequestCtx) {
				for _, m := range middlewares {
					m(ctx)
				}

				proxyServer.ServeHTTP(ctx)
			})

			go func() {
				addr := fmt.Sprintf(":%d", server.Port)
				Infof("Starting http server at %s", addr)
				err := fasthttp.ListenAndServe(addr, r.Handler)
				if err != nil {
					Fatal(err)
				}
			}()
		}
	}

	if config.WebSocket != nil {
		for i := 0; i < len(config.WebSocket.Servers); i++ {
			var server = config.WebSocket.Servers[i]

			var options = make([]proxy.OptionWS, len(server.Upstream))

			for j := 0; j < len(server.Upstream); j++ {
				var upstream = server.Upstream[j]
				options[j] = proxy.WithURL_OptionWS(fmt.Sprintf("ws://%s:%d", upstream.Host, upstream.Port))
			}

			go func() {
				proxyServer, _ := proxy.NewWSReverseProxyWith(options...)
				addr := fmt.Sprintf(":%d", server.Port)
				Infof("Starting websocket server at %s", addr)

				if err := fasthttp.ListenAndServe(addr, func(ctx *fasthttp.RequestCtx) {
					proxyServer.ServeHTTP(ctx)
				}); err != nil {
					Fatal(err)
				}
			}()
		}
	}

	for {
		CheckLoggerChannel()
	}
}
