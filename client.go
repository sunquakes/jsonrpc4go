package jsonrpc4go

import (
	"errors"
	"reflect"
	"strings"

	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Create a new JSON-RPC client
 * @Param name: Service name
 * @Param protocol: Protocol type (http/https/tcp)
 * @Param server: Service address (string) or service discovery driver (discovery.Driver)
 * @Return client.Client: Client interface
 * @Return error: Error message
 */
func NewClient(name string, protocol string, server any) (client.Client, error) {
	var p client.Protocol
	var (
		address string
		dc      discovery.Driver
	)
	if reflect.TypeOf(server).Kind() == reflect.String {
		address = server.(string)
	} else {
		dc = server.(discovery.Driver)
	}

	switch strings.ToLower(protocol) {
	case "http":
		p = &client.Http{Name: name, Protocol: protocol, Address: address, Discovery: dc}
	case "https":
		p = &client.Http{Name: name, Protocol: protocol, Address: address, Discovery: dc}
	case "tcp":
		p = &client.Tcp{Name: name, Protocol: protocol, Address: address, Discovery: dc}
	default:
		return nil, errors.New("the protocol can not be supported")
	}
	return client.NewClient(p), nil
}
