package servers

import (
	"github.com/sunquakes/jsonrpc4go/discovery"
)

type Servers string

func NewServers(rawURL string) (discovery.Driver, error) {
	servers := Servers(rawURL)
	return &servers, nil
}

func (d *Servers) Register(name string, protocol string, hostname string, port int) error {
	return nil
}

func (d *Servers) Get(name string) (string, error) {
	return string(*d), nil
}
