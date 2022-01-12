package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/MagicFun1241/fasthttp-reverse-proxy/v2"
	"github.com/MagicFun1241/roxy/core"
	"github.com/MagicFun1241/roxy/core/js/logger"
	pluginsCore "github.com/MagicFun1241/roxy/plugins/core"
	"github.com/dop251/goja"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	log "unknwon.dev/clog/v2"
)

func init() {
	err := log.NewConsole()
	if err != nil {
		panic("unable to create new logger: " + err.Error())
	}
}

func main() {
	configPath := flag.String("config", "config.yml", "config file path")
	flag.Parse()

	proxy.SetProduction()

	config := Config{}
	configBytes, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal("Error reading config file")
		return
	}

	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatal("Error processing config file")
		return
	}

	configBytes = nil

	vm := goja.New()
	pluginsCore.RegisterModule(vm)
	logger.Register(vm)

	if config.Plugins != nil {
		for _, plugin := range config.Plugins {
			p := path.Join("plugins", fmt.Sprintf("%s.js", plugin))
			if _, err := os.Stat(p); os.IsNotExist(err) {
				log.Warn("Plugin '%s' file is not exists", plugin)
			}

			pluginCode, _ := ioutil.ReadFile(p)
			_, err = vm.RunString(string(pluginCode))

			if err != nil {
				log.Error(err.Error())
			}
		}
	}

	if config.Dns != nil {
		if !validateIp(*config.Dns) {
			log.Fatal("Invalid DNS")
		}
	} else {
		var dns = "1.1.1.1"
		config.Dns = &dns
	}
	log.Info("DNS registered")

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
							log.Fatal("Invalid ip or mask passed for '%s' allowedHostsGroup\n", groupName)
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
		var usesDefaultPort []*httpServer

		var filesystemHandlers = make(map[string]fasthttp.RequestHandler)

		if config.Http.Servers != nil {
			for serverIndex, server := range config.Http.Servers {
				var proxyServer *proxy.ReverseProxy
				var middlewares []func(ctx *fasthttp.RequestCtx)

				if server.Port == 0 || server.Port == 80 {
					if len(server.Upstream) > 1 {
						log.Error("Servers on default port must contain single upstream")
						return
					}

					usesDefaultPort = append(usesDefaultPort, &server)
					continue
				}

				if server.AllowedHostsGroups != nil {
					for _, group := range server.AllowedHostsGroups {
						if config.Security.AllowedHostsGroups[group] == nil {
							log.Fatal("Item refers to not exist group '%s'\n", group)
							return
						}
					}
				}

				if len(server.Upstream) == 1 {
					var upstream = server.Upstream[0]
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
							log.Info("Starting http3 server at %s", addr)

							httpProxy := httputil.NewSingleHostReverseProxy(originUrl)
							err = http3.ListenAndServeQUIC(addr, *server.QuicCertificate, *server.QuicKey, httpProxy)
							if err != nil {
								log.Fatal(err.Error())
							}
						}()
					}

					proxyServer = proxy.NewReverseProxy(upstream.Host + ":" + strconv.Itoa(int(upstream.Port)))
				}

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
								log.Error("Connection from %s to :%d is not allowed with groups %v", ctx.RemoteIP().String(), server.Port, server.AllowedHostsGroups)
								ctx.Error("Access denied", fasthttp.StatusForbidden)
							}
						}
					})
				}

				serverIndex := serverIndex
				go func(port uint16, routes []httpRoute) {
					addr := fmt.Sprintf(":%d", port)
					log.Info("Starting http server at %s", addr)
					err := fasthttp.ListenAndServe(addr, func(ctx *fasthttp.RequestCtx) {
						urlPath := string(ctx.Path())

						for routeIndex, route := range routes {
							if route.ToStatic != (routeStatic{}) && strings.HasPrefix(urlPath, route.Value) {
								startOffset := strings.LastIndex(urlPath, route.Value)
								if startOffset == 0 {
									startOffset = len(route.Value)
								}

								newPath := urlPath[startOffset:]
								ctx.Request.URI().SetPath(newPath)

								key := core.FormatFilesystemHandlerKey(core.LocationGeneral, serverIndex, routeIndex)
								if filesystemHandlers[key] == nil {
									fs := &fasthttp.FS{
										Root:               route.ToStatic.Root,
										IndexNames:         []string{"index.html"},
										GenerateIndexPages: route.ToStatic.IndexPages,
									}

									filesystemHandlers[key] = fs.NewRequestHandler()
								}

								filesystemHandlers[key](ctx)
								return
							} else if route.To != "" && strings.HasPrefix(urlPath, route.Value) {
								for _, m := range middlewares {
									m(ctx)
								}

								proxyServer.ServeHTTP(ctx)
								return
							}
						}

						ctx.Error("Not found", fasthttp.StatusNotFound)
					})
					if err != nil {
						log.Fatal(err.Error())
					}
				}(server.Port, server.Routes)
			}
		}

		if config.Http.Default != nil || len(usesDefaultPort) > 0 {
			log.Info("Starting default http proxy")

			var httpProxy *proxy.ReverseProxy

			if config.Http.Default != nil {
				httpProxy = proxy.NewReverseProxy(fmt.Sprintf(":%d", config.Http.Default.Port))
			}

			var otherProxies map[string]*proxy.ReverseProxy
			if len(usesDefaultPort) > 0 {
				otherProxies = make(map[string]*proxy.ReverseProxy, len(usesDefaultPort))
			}

			for _, s := range usesDefaultPort {
				if s.Upstream == nil {
					continue
				}

				if !validateIp(s.Upstream[0].Host) {
					s.Upstream[0].Host = LookupIp(ipResolver, s.Upstream[0].Host)
				}

				origin := fmt.Sprintf("%s:%d", s.Upstream[0].Host, s.Upstream[0].Port)
				otherProxies[origin] = proxy.NewReverseProxy(origin)
			}

			go func() {
				err = fasthttp.ListenAndServe(":80", func(ctx *fasthttp.RequestCtx) {
					host := string(ctx.Request.Header.Peek("Host"))
					if host == "" {
						ctx.Error("Unknown domain", fasthttp.StatusForbidden)
						return
					}

					if len(usesDefaultPort) > 0 {
						for serverIndex, s := range usesDefaultPort {
							for _, d := range s.Domains {
								if d == host {
									if s.Routes != nil {
										urlPath := string(ctx.Path())

										for routeIndex, r := range s.Routes {
											if r.ToStatic != (routeStatic{}) && strings.HasPrefix(urlPath, r.Value) {
												startOffset := strings.LastIndex(urlPath, r.Value)
												if startOffset == 0 {
													startOffset = len(r.Value)
												}

												newPath := urlPath[startOffset:]
												log.InfoTo("request", "%s", string(ctx.Request.URI().Path()))

												ctx.Request.URI().SetPath(newPath)

												key := core.FormatFilesystemHandlerKey(core.LocationDefault, serverIndex, routeIndex)
												if filesystemHandlers[key] == nil {
													fs := &fasthttp.FS{
														Root:               r.ToStatic.Root,
														IndexNames:         []string{"index.html"},
														GenerateIndexPages: r.ToStatic.IndexPages,
													}

													filesystemHandlers[key] = fs.NewRequestHandler()
												}

												filesystemHandlers[key](ctx)
												return
											} else if r.To != "" && strings.HasPrefix(urlPath, r.Value) {
												ctx.Request.URI().SetPath(path.Join(r.To, urlPath))
											}
										}

										ctx.Error("Not found", fasthttp.StatusNotFound)
										return
									}
								}

								origin := fmt.Sprintf("%s:%d", s.Upstream[0].Host, s.Upstream[0].Port)
								otherProxies[origin].ServeHTTP(ctx)
								return
							}
						}
					} else if httpProxy != nil {
						httpProxy.ServeHTTP(ctx)
						return
					}

					ctx.Error("No destination", fasthttp.StatusForbidden)
				})
				if err != nil {
					return
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
				log.Info("Starting websocket server at %s", addr)

				if err := fasthttp.ListenAndServe(addr, func(ctx *fasthttp.RequestCtx) {
					proxyServer.ServeHTTP(ctx)
				}); err != nil {
					log.Fatal(err.Error())
				}
			}()
		}
	}

	for {
	}
}
