package api

import (
	"fmt"
	"net"
	"strconv"
)

type Endpoint struct {
	Protocol string
	Host     string
	Port     uint
	Path     string
}

func NewEndpoint(address string) *Endpoint {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	p, _ := strconv.ParseUint(port, 10, 64)
	return &Endpoint{
		Host: host,
		Port: uint(p),
	}
}

func (e *Endpoint) String() string {
	var protocol string
	if e.Protocol != "" {
		protocol = fmt.Sprintf("%s://", e.Protocol)
	}
	return fmt.Sprintf("%s%s%s", protocol, e.Address(), e.Path)
}

func (e *Endpoint) Address() string {
	var port string
	if e.Port > 0 {
		port = fmt.Sprintf(":%d", e.Port)
	}
	return fmt.Sprintf("%s%s", e.Host, port)
}
