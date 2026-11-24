package jsonrpc4go

import (
	"encoding/json"
	"errors"
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
	switch v := server.(type) {
	case string:
		address = v
	case discovery.Driver:
		dc = v
	default:
		return nil, errors.New("the server must be an address string or a discovery.Driver")
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

/**
 * @Description: Call executes a JSON-RPC method and decodes the result into T
 * @Param c: Client interface
 * @Param method: Method name
 * @Param args: Positional method arguments
 * @Return T: Decoded result value
 * @Return error: Error message
 */
func Call[T any](c client.Client, method string, args ...any) (T, error) {
	var zero T
	res, err := c.Call(method, args...)
	if err != nil {
		return zero, err
	}
	if res == nil {
		return zero, nil
	}
	e, err := json.Marshal(res)
	if err != nil {
		return zero, err
	}
	err = json.Unmarshal(e, &zero)
	return zero, err
}
