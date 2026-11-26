package main

import (
	"fmt"

	"github.com/sunquakes/jsonrpc4go/v2"
)

func main() {
	// The client performs the MCP initialize handshake once, then speaks
	// JSON for unary results and reads SSE for streaming results.
	c, _ := jsonrpc4go.NewClient("IntRpc", "streamable-http", "127.0.0.1:3232")
	result1, err1 := jsonrpc4go.Call[int](c, "Add", 1, 6) // unary JSON response
	fmt.Println(err1)                                     // nil
	fmt.Println(result1)                                  // 7

	// notify is answered with 202 Accepted
	fmt.Println(c.Notify("Add", 2, 3)) // nil

	// streaming: values arrive one by one as SSE events
	sc, _ := jsonrpc4go.NewClient("StreamRpc", "streamable-http", "127.0.0.1:3232")
	for v, err := range jsonrpc4go.Stream[int](sc, "Count", 3) {
		fmt.Println(v, err) // 1 <nil>  2 <nil>  3 <nil>
	}

	// Call also understands streams and returns them as []any
	values, err := sc.Call("Count", 2)
	fmt.Println(values, err) // [1 2] <nil>
}
