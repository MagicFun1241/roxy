package main

type upstreamServerWithWeight struct {
	Port   uint32
	Host   string
	Weight uint8
}

type upstreamServer struct {
	Port uint32
	Host string
}

type httpServer struct {
	Name     string
	Port     uint32
	Domains  []string
	Upstream []upstreamServerWithWeight
}

type webSocketServer struct {
	Name     string
	Port     uint32
	Upstream []upstreamServer `yaml:",flow"`
}

type Config struct {
	Dns *string

	Http *struct {
		Servers []httpServer `yaml:",flow"`
	}

	WebSocket *struct {
		Servers []webSocketServer `yaml:",flow"`
	}
}
