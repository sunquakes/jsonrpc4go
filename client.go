package jsonrpc4go

import (
	"encoding/json"
	"errors"
	"iter"
	"strings"

	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Create a new JSON-RPC client
 * @Param name: Service name
 * @Param protocol: Protocol type (http/https/tcp/streamable-http)
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
	case "streamable-http":
		p = &client.StreamableHttp{Name: name, Address: address, Discovery: dc}
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

/**
 * @Description: Stream executes a JSON-RPC method on a streaming-capable
 * client and decodes each streamed value into T
 * @Param c: Client interface (must also implement client.Streamer)
 * @Param method: Method name
 * @Param args: Positional method arguments
 * @Return iter.Seq2[T, error]: Sequence of decoded values and errors
 */
func Stream[T any](c client.Client, method string, args ...any) iter.Seq2[T, error] {
	streamer, ok := c.(client.Streamer)
	if !ok {
		return func(yield func(T, error) bool) {
			var zero T
			yield(zero, errors.New("the client does not support streaming"))
		}
	}
	var zero T
	return func(yield func(T, error) bool) {
		for v, err := range streamer.Stream(method, args...) {
			if err != nil {
				if !yield(zero, err) {
					return
				}
				continue
			}
			e, err := json.Marshal(v)
			if err != nil {
				if !yield(zero, err) {
					return
				}
				continue
			}
			var t T
			if err := json.Unmarshal(e, &t); err != nil {
				if !yield(zero, err) {
					return
				}
				continue
			}
			if !yield(t, nil) {
				return
			}
		}
	}
}
