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
- Server
```go
package main

import (
    "github.com/sunquakes/jsonrpc4go"
)

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

func (i *IntRpc) Add(params *Params, result *int) error {
	*result = params.A + params.B
	return nil
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

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result2 struct {
	C int `json:"c"`
}

func main() {
	result := new(int)
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3232")
	err := c.Call("Add", Params{1, 6}, result, false)
	// data sent: {"id":"1604283212", "jsonrpc":"2.0", "method":"IntRpc/Add", "params":{"a":1,"b":6}}
	// data received: {"id":"1604283212", "jsonrpc":"2.0", "result":7}
	fmt.Println(err) // nil
	fmt.Println(*result) // 7
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
s.SetBeforeFunc(func(id interface{}, method string, params interface{}) error {
    // If the function returns an error, the program stops execution and returns an error message to the client
    // return errors.New("Custom Error")
    return nil
})
// Set the hook function of after method execution
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
// notify
result2 := new(Result2)
err2 := c.Call("Add2", Params{1, 6}, result2, true)
// data sent: {"jsonrpc":"2.0","method":"IntRpc/Add2","params":{"a":1,"b":6}}
// data received: {"jsonrpc":"2.0","result":{"c":7}}
fmt.Println(err2) // nil
fmt.Println(*result2) // {7}
```
- Batch call
```go
// batch call
result3 := new(int)
err3 := c.BatchAppend("Add1", Params{1, 6}, result3, false)
result4 := new(int)
err4 := c.BatchAppend("Add", Params{2, 3}, result4, false)
c.BatchCall()
// data sent: [{"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add1","params":{"a":1,"b":6}},{"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add","params":{"a":2,"b":3}}]
// data received: [{"id":"1604283212","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}},{"id":"1604283212","jsonrpc":"2.0","result":5}]
fmt.Println((*err3).Error()) // Method not found
fmt.Println(*result3) // 0
fmt.Println(*err4) // nil
fmt.Println(*result4) // 5
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
