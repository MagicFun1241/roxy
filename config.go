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

type defaultHttpServer struct {
	Port uint32
}

type httpServer struct {
	Name               string
	Port               *uint32
	Domains            []string
	AllowedHostsGroups []string
	Upstream           []upstreamServerWithWeight

	Quic            *bool
	QuicCertificate *string `yaml:"quicCertificate"`
	QuicKey         *string `yaml:"quicKey"`
}

type webSocketServer struct {
	Name               string
	Port               uint32
	AllowedHostsGroups []string
	Upstream           []upstreamServer `yaml:",flow"`
}

type security struct {
	AllowedHostsGroups map[string][]string `yaml:"allowedHostsGroups"`
}

type Config struct {
	Dns *string

	Security *security

	Http *struct {
		Default *defaultHttpServer
		Servers []httpServer `yaml:",flow"`
	}

	WebSocket *struct {
		Servers []webSocketServer `yaml:",flow"`
	}
}
