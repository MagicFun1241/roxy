package main

type upstreamServerWithWeight struct {
	Port   uint32
	Host   string
	Weight *uint8
}

type upstreamServer struct {
	Port uint16
	Host string
}

type defaultHttpServer struct {
	Port uint16
}

type httpRoute struct {
	Value    string
	To       string
	ToStatic routeStatic `yaml:"toStatic"`
}

type httpServer struct {
	Name               string
	Port               uint16
	Domains            []string
	AllowedHostsGroups []string
	Upstream           []upstreamServerWithWeight
	Routes             []httpRoute

	Quic            *bool
	QuicCertificate *string `yaml:"quicCertificate"`
	QuicKey         *string `yaml:"quicKey"`
}

type webSocketServer struct {
	Name               string
	Port               uint16
	AllowedHostsGroups []string
	Upstream           []upstreamServer `yaml:",flow"`
}

type security struct {
	AllowedHostsGroups map[string][]string `yaml:"allowedHostsGroups"`
}

type routeStatic struct {
	Root       string
	IndexPages bool `yaml:"indexPages"`
}

type Config struct {
	Dns *string

	Plugins []string

	Security *security

	Http *struct {
		Default *defaultHttpServer
		Servers []httpServer `yaml:",flow"`
	}

	WebSocket *struct {
		Servers []webSocketServer `yaml:",flow"`
	}
}
