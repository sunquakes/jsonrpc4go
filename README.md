English | [🇨🇳中文](README_ZH.md)
# jsonrpc4go

> A high-performance [JSON-RPC 2.0](https://www.jsonrpc.org/specification) implementation in Go. Supports HTTP/TCP, service registration & discovery (Consul / Nacos / etcd), client-side load-balancing, rate limiting, hooks, batch calls and notifications.

JSON-RPC 2.0 is the wire protocol that powers **[MCP (Model Context Protocol)](https://modelcontextprotocol.io/)** — the standard interface between LLMs and external tools / data sources. Because `jsonrpc4go` speaks this protocol natively over both HTTP and TCP, it is an ideal foundation for building **MCP servers and clients** in Go, letting AI agents call your services as first-class tools.

## 🤖 Use as an MCP framework
[Model Context Protocol (MCP)](https://modelcontextprotocol.io/) standardizes how AI assistants discover and invoke external tools, resources and prompts. Its transport layer is exactly JSON-RPC 2.0 — the same protocol `jsonrpc4go` is built on. This makes `jsonrpc4go` a ready-to-use foundation for AI-native services:

- **Expose Go functions as MCP tools** — register a struct method and it becomes an AI-callable tool with `method` = `ServiceName/Method`.
- **Bidirectional role** — act as an MCP server (tools provider) or an MCP client (agent that calls remote tools) using the same `Server` / `Client` APIs.
- **Transport flexibility** — serve MCP over HTTP for stateless gateways, or over TCP for persistent, streaming-friendly agent connections.
- **Production-ready foundations** — service discovery (Consul / Nacos / etcd), client-side load-balancing, rate limiting and before/after hooks let you run MCP tool services at scale, with audit / auth / tracing hooks for safe agent access.
- **Batch calls & notifications** — map naturally to MCP's batched tool invocations and one-way notifications.

```go
// Any registered method is already an AI-callable tool over JSON-RPC 2.0.
s, _ := jsonrpc4go.NewServer("http", 3232)
s.Register(new(IntRpc)) // IntRpc.Add is now callable by any MCP client
s.Start()
```

## 🧰 Installing
```
go get -u github.com/sunquakes/jsonrpc4go
```
## 📖 Getting started
Server methods take value arguments and return the result directly (optionally followed by an `error`). Arguments travel as a positional JSON array (`"params":[1,6]`).

- Server
```go
package main

import (
    "github.com/sunquakes/jsonrpc4go"
)

type IntRpc struct{}

func (i *IntRpc) Add(a, b int) int {
	return a + b
}

func main() {
	s, _ := jsonrpc4go.NewServer("http", 3232) // the protocol is http
	s.Register(new(IntRpc))
	s.Start()
}
```
- Client
```go
package main

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go"
)

func main() {
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3232")
	result, err := jsonrpc4go.Call[int](c, "Add", 1, 6)
	// data sent: {"id":"1","jsonrpc":"2.0","method":"IntRpc/Add","params":[1,6]}
	// data received: {"id":"1","jsonrpc":"2.0","result":7}
	fmt.Println(err) // nil
	fmt.Println(result) // 7
}
```
## ⚔️ Test
```
go test -v ./test/...
```
## 🚀 More features
- TCP protocol
```go
s, _ := jsonrpc4go.NewServer("tcp", 3232) // the protocol is tcp

c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3232") // the protocol is tcp
```
- Hooks (Add the following code before 's.Start()')
```go
// Set the hook function of before method execution
// params is a []any of the decoded positional arguments, e.g. []any{1, 6}
s.SetBeforeFunc(func(id interface{}, method string, params interface{}) error {
    // If the function returns an error, the program stops execution and returns an error message to the client
    // return errors.New("Custom Error")
    return nil
})
// Set the hook function of after method execution
// result is the value returned by the method, e.g. 7
s.SetAfterFunc(func(id interface{}, method string, result interface{}) error {
    // If the function returns an error, the program stops execution and returns an error message to the client
    // return errors.New("Custom Error")
    return nil
})
```
- Rate limit (Add the following code before 's.Start()')
```go
s.SetRateLimit(20, 10) //The maximum concurrent number is 10, The maximum request speed is 20 times per second
```
- Custom package EOF when the protocol is tcp
```go
// Add the following code before 's.Start()'
s.SetOptions(server.TcpOptions{"aaaaaa", nil}) // Custom package EOF when the protocol is tcp
// Add the following code before 'c.Call()' or 'c.BatchCall()'
c.SetOptions(client.TcpOptions{"aaaaaa", nil}) // Custom package EOF when the protocol is tcp
```
- Notify
```go
// notify: no result is returned, only an error
err := c.Notify("Add2", 1, 6)
// data sent: {"jsonrpc":"2.0","method":"IntRpc/Add2","params":[1,6]}
fmt.Println(err) // nil
```
- Batch call
```go
// batch call
c.BatchAppend("Add1", 1, 6)
c.BatchAppend("Add", 2, 3)
results, err := c.BatchCall()
// data sent: [{"id":"1","jsonrpc":"2.0","method":"IntRpc/Add1","params":[1,6]},{"id":"2","jsonrpc":"2.0","method":"IntRpc/Add","params":[2,3]}]
// data received: [{"id":"1","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}},{"id":"2","jsonrpc":"2.0","result":5}]
fmt.Println(err) // <nil>
fmt.Println(results[0].Err) // Method not found
fmt.Println(results[1].Result) // 5
```
- Client-Side Load-Balancing
```go
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3232,127.0.0.1:3233,127.0.0.1:3234")
```

## Service registration & discovery
### Consul
```go
/**
 * check: true or false. The switch of the health check.
 * interval: The interval of the health check. For example: 10s.
 * timeout: Timeout. For example: 10s.
 * instanceId: Instance ID. Distinguish the same service in different nodes. For example: 1.
 */
dc, _ := consul.NewConsul("http://localhost:8500?check=true&instanceId=1&interval=10s&timeout=10s")

// Set in the server.
s, _ := jsonrpc4go.NewServer("tcp", 3614)
// If the default node ip is used, the second parameter can be set ""
s.SetDiscovery(dc, "127.0.0.1")
s.Register(new(IntRpc))
s.Start()

// Set in the client
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
```
### Nacos
```go
dc, _ := nacos.NewNacos("http://127.0.0.1:8849")

// Set in the server.
s, _ := jsonrpc4go.NewServer("tcp", 3616)
// If the default node ip is used, the second parameter can be set ""
s.SetDiscovery(dc, "127.0.0.1")
s.Register(new(IntRpc))
s.Start()

// Set in the client
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
```

## 📄 License
Source code in `jsonrpc4go` is available under the [Apache-2.0 license](/LICENSE).
