package main

import (
	"flag"
	"github.com/valyala/fasthttp"
	reverseProxy "github.com/yeqown/fasthttp-reverse-proxy"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"strconv"
)

type HttpUpstreamServer struct {
	Port uint32
	Host string
	Weight uint
}

type WebSocketUpstreamServer struct {
	Port uint32
	Host string
}

type HttpServer struct {
	Name string
	Port uint32
	Domains []string
	Upstream []HttpUpstreamServer
}

type WebSocketServer struct {
	Name string
	Port uint32
	Upstream []WebSocketUpstreamServer `yaml:",flow"`
}

type Config struct {
	Dns *string

	Http *struct {
		Servers []HttpServer `yaml:",flow"`
	}

	WebSocket *struct{
		Servers []WebSocketServer `yaml:",flow"`
	}
}

func main() {
	configPath := flag.String("config", "config.yml", "config file path")

	flag.Parse()

	go startHttpServer(0)
	go startHttpServer(1)

	config := Config{}
	configBytes, err := ioutil.ReadFile(*configPath)

	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.Fatalf("error: %v", err)
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

	if config.Http != nil {
		for i := 0; i < len(config.Http.Servers); i++ {
			var proxyServer *reverseProxy.ReverseProxy

			if len(config.Http.Servers[i].Upstream) == 1 {
				var upstream = config.Http.Servers[i].Upstream[0]
				if !validateIp(upstream.Host) {
					upstream.Host = lookupIp(upstream.Host, *config.Dns)
				}

				proxyServer = reverseProxy.NewReverseProxy(upstream.Host + ":" + strconv.Itoa(int(upstream.Port)))
			} else {
				weights := make(map[string]reverseProxy.Weight)

				for j := 0; j < len(config.Http.Servers[i].Upstream); j++ {
					var upstream = config.Http.Servers[i].Upstream[j]
					if !validateIp(upstream.Host) {
						upstream.Host = lookupIp(upstream.Host, *config.Dns)
					}

					if j == 0 && len(weights) == 2 {
						addr := upstream.Host+":"+strconv.Itoa(int(upstream.Port))
						weights[addr] = reverseProxy.Weight(upstream.Weight-1)

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

			err := fasthttp.ListenAndServe(":"+strconv.Itoa(int(config.Http.Servers[i].Port)), func (ctx *fasthttp.RequestCtx) {
				proxyServer.ServeHTTP(ctx)
			})

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
				options = append(options, reverseProxy.WithURL_OptionWS("ws://"+upstream.Host+":"+ strconv.Itoa(int(upstream.Port))))
			}

			proxyServer, _ := reverseProxy.NewWSReverseProxyWith(options...)

			if err := fasthttp.ListenAndServe(":"+strconv.Itoa(int(server.Port)), func(ctx *fasthttp.RequestCtx) {
				proxyServer.ServeHTTP(ctx)
			}); err != nil {
				log.Fatal(err)
			}
		}
	}

	config = Config{}

	log.Print("Roxy started")
}
