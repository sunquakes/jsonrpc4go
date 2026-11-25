[English](README.md) | 🇨🇳中文
# jsonrpc4go

> 一个高性能的 [JSON-RPC 2.0](https://www.jsonrpc.org/specification) Go 语言实现。支持 HTTP/TCP/Streamable HTTP（MCP）、服务注册与发现（Consul / Nacos / etcd）、客户端负载均衡、限流、钩子、批量请求、通知与流式结果。

JSON-RPC 2.0 正是 **[MCP（Model Context Protocol，模型上下文协议）](https://modelcontextprotocol.io/)** 的传输协议 —— 也就是 LLM 与外部工具 / 数据源之间的标准接口。由于 `jsonrpc4go` 原生支持基于 HTTP 与 TCP 的该协议，它是用 Go 构建 **MCP 服务端与客户端** 的理想底层框架，让 AI Agent 能够把你的服务当作一等公民的工具来调用。

## 🤖 作为 MCP 框架使用
[模型上下文协议（MCP）](https://modelcontextprotocol.io/) 规范了 AI 助手如何发现并调用外部工具、资源和提示词。它的传输层正是 JSON-RPC 2.0 —— 与 `jsonrpc4go` 所基于的协议完全一致。这让 `jsonrpc4go` 成为构建 AI 原生服务的现成底座：

- **把 Go 函数暴露为 MCP 工具** —— 注册一个结构体方法，它就变成了一个 AI 可调用的工具，`method` = `ServiceName/Method`。
- **双向角色** —— 同一套 `Server` / `Client` API，既可作为 MCP 服务端（工具提供方），也可作为 MCP 客户端（调用远程工具的 Agent）。
- **传输层灵活** —— 通过 HTTP 提供无状态网关型 MCP 服务，或通过 TCP 提供持久、对流式友好的 Agent 连接。
- **生产级基础设施** —— 服务发现（Consul / Nacos / etcd）、客户端负载均衡、限流以及前后置钩子，让 MCP 工具服务可大规模运行；钩子可用于审计 / 鉴权 / 链路追踪，保障 Agent 访问安全。
- **批量请求与通知** —— 天然映射 MCP 的批量工具调用和单向通知。

```go
// 任何已注册的方法，本身就是一个基于 JSON-RPC 2.0 的 AI 可调用工具。
s, _ := jsonrpc4go.NewServer("http", 3232)
s.Register(new(IntRpc)) // IntRpc.Add 现在可被任意 MCP 客户端调用
s.Start()
```

## 🌊 Streamable HTTP 传输（兼容 MCP）
`streamable-http` 协议把所有流量收敛到单一 `/mcp` 端点，并遵循
[MCP Streamable HTTP 传输规范](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)，
任何 MCP Agent 客户端都可以直连：

- POST 请求：一元结果以 JSON 返回，流式结果以 Server-Sent Events（SSE）推送。
- 支持 `Mcp-Session-Id` 会话与 `Mcp-Protocol-Version` 版本协商（`2024-11-05` / `2025-03-26` / `2025-06-18` / `2025-11-25`）。
- 支持 `Last-Event-ID` 事件重放、GET 服务端→客户端 SSE 通道、DELETE 结束会话。
- 通知请求返回 `202`；浏览器 `Origin` 白名单校验失败返回 `403`。
- 可选的 `Host` 白名单（`AllowedHosts`）会以 `421` 拒绝其他主机，防止针对本地
  服务器的 DNS rebinding 攻击。

返回切片、数组、channel 或迭代器（`iter.Seq` / `iter.Seq2`）的方法，在该传输上会逐值推送；
tcp/http 传输则将其收集为数组返回。

- 服务端
```go
type StreamRpc struct{}

func (s *StreamRpc) Count(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

s, _ := jsonrpc4go.NewServer("streamable-http", 3232)
s.Register(new(IntRpc))
s.Register(new(StreamRpc))
s.SetOptions(server.StreamableHttpOptions{
	AllowedOrigins: []string{"https://client.example"},
	AllowedHosts:   []string{"localhost", "127.0.0.1"}, // 可选的 DNS rebinding 防护
})
s.Start() // 提供 http://0.0.0.0:3232/mcp 服务
```

- 客户端（自动完成一次 MCP `initialize` 握手，随后同时兼容 JSON 与 SSE 响应）
```go
c, _ := jsonrpc4go.NewClient("StreamRpc", "streamable-http", "127.0.0.1:3232")

// 流式：值随 SSE 事件逐个到达。
for v, err := range jsonrpc4go.Stream[int](c, "Count", 3) {
	fmt.Println(v, err) // 1 <nil>、2 <nil>、3 <nil>
}

// Call 同时理解两种形态：流式结果以 []any 返回。
values, _ := c.Call("Count", 2) // [1, 2]
```

- MCP 兼容层：同一端点上实现了 `initialize`、`ping`、`tools/list` 与 `tools/call`
  （支持 `_meta.progressToken` 进度通知）。每个已注册方法都会以 `Service_Method`
  的名字发布为工具；`inputSchema` 通过反射生成（Go 无法读取参数名，值参数命名为
  `arg0..argN`，结构体参数则展开为其 JSON 标签字段）。旧的 `Service/method`
  JSON-RPC 分发保持完全兼容。
- 一致性测试：CI 使用官方
  [`modelcontextprotocol/conformance`](https://github.com/modelcontextprotocol/conformance)
  server 套件对示例服务器进行校验，**全部 active 场景通过**（协议修订至
  `2025-11-25`）：initialize/ping、tools（list/call/进度）、resources
  （list/read/templates/subscribe）、prompts、completion、logging、服务端发起的
  sampling 与 elicitation（SEP-1034 默认值、SEP-1330 枚举）、JSON Schema
  2020-12 关键字保留、SSE priming 与 Last-Event-ID 恢复、DNS rebinding 防护。
  `conformance-baseline.yml` 有意留空；未来任何缺口必须显式登记，否则构建失败。

## 🧰 安装
```
go get -u github.com/sunquakes/jsonrpc4go
```
## 📖 开始使用
服务端方法直接接收值参数并返回结果（可选地在其后返回一个 `error`）。参数以位置数组的形式传输（`"params":[1,6]`）。

- 服务端代码
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
	s, _ := jsonrpc4go.NewServer("http", 3232) // http协议
	s.Register(new(IntRpc))
	s.Start()
}
```
- 客户端代码
```go
package main

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go"
)

func main() {
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3232") // http协议
	result, err := jsonrpc4go.Call[int](c, "Add", 1, 6)
	// 发送的数据格式: {"id":"1","jsonrpc":"2.0","method":"IntRpc/Add","params":[1,6]}
	// 接收的数据格式: {"id":"1","jsonrpc":"2.0","result":7}
	fmt.Println(err) // nil
	fmt.Println(result) // 7
}
```
## ⚔️ 测试
```
go test -v ./test/...
```
## 🚀 更多特性
- tcp协议
```go
s, _ := jsonrpc4go.NewServer("tcp", 3232) // tcp协议

c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3232") // tcp协议
```
- 钩子 (在代码's.Start()'前添加下面的代码)
```go
// 在方法前执行的钩子方法
// params 为解码后的位置参数列表 []any，例：[]any{1, 6}
s.SetBeforeFunc(func(id interface{}, method string, params interface{}) error {
    // 如果方法返回error类型，服务端停止执行并返回错误信息到客户端
    // 例：return errors.New("Custom Error")
    return nil
})
// 在方法后执行的钩子方法
// result 为方法直接返回的值，例：7
s.SetAfterFunc(func(id interface{}, method string, result interface{}) error {
    // 如果方法返回error类型，服务端停止执行并返回错误信息到客户端
    // 例：return errors.New("Custom Error")
    return nil
})
```
- 限流 (在代码's.Start()'前添加下面的代码)
```go
s.SetRateLimit(20, 10) // 最大并发数为10, 最大请求数为每秒20个
```
- tcp协议时自定义请求结束符
```go
// 在代码's.Start()'前添加下面的代码
s.SetOptions(server.TcpOptions{"aaaaaa", nil}) // 仅tcp协议生效
// 在代码'c.Call()'或'c.BatchCall()'前添加下面的代码
c.SetOptions(client.TcpOptions{"aaaaaa", nil}) // 仅tcp协议生效
```
- 通知请求
```go
// 通知：不返回结果，只返回错误
err := c.Notify("Add2", 1, 6)
// 发送的数据格式: {"jsonrpc":"2.0","method":"IntRpc/Add2","params":[1,6]}
fmt.Println(err) // nil
```
- 批量请求
```go
// 批量请求
c.BatchAppend("Add1", 1, 6)
c.BatchAppend("Add", 2, 3)
results, err := c.BatchCall()
// 发送的数据格式: [{"id":"1","jsonrpc":"2.0","method":"IntRpc/Add1","params":[1,6]},{"id":"2","jsonrpc":"2.0","method":"IntRpc/Add","params":[2,3]}]
// 接收的数据格式: [{"id":"1","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}},{"id":"2","jsonrpc":"2.0","result":5}]
fmt.Println(err) // <nil>
fmt.Println(results[0].Err) // Method not found
fmt.Println(results[1].Result) // 5
```
- 用户端负载均衡
```go
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3232,127.0.0.1:3233,127.0.0.1:3234")
```
- 绑定方法为强类型函数
```go
// 一次性绑定客户端、方法名与结果类型
add := jsonrpc4go.Bind[int](c, "Add")
add2 := jsonrpc4go.Bind[Result](c, "Add2")

sum, err := add(1, 6)   // 7, <nil>
res, err := add2(2, 3)  // {5}, <nil>
```
- 类型化客户端代码生成（`rpcgen`）
```go
// api.go：用接口描述远端服务
//go:generate go run github.com/sunquakes/jsonrpc4go/cmd/rpcgen -iface IntRpcApi -service IntRpc -out intrpc_rpc.go

type IntRpcApi interface {
    Add(a, b int) (int, error)
    Add2(a, b int) (Result, error)
}

// main.go：执行 `go generate ./...` 后，每次调用都是普通的强类型 Go 调用
c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3232")
api := NewIntRpcApiClient(c)
sum, err := api.Add(1, 6) // 7, <nil>
```
每个方法必须返回 `(T, error)` 或 `error`；可变参数方法、泛型方法和内嵌接口会在生成阶段直接报错，错误在 `go generate` 时暴露，而不是等到运行时。

## 服务注册和发现
### Consul
```go
/**
 * check: true或者false, 开启健康检查
 * interval: 健康检查周期，例：10s
 * timeout: 请求超时时间，例：10s
 * instanceId: 实例ID，同一服务多负载时区分用，例：1
 */
dc, _ := consul.NewConsul("http://localhost:8500?check=true&instanceId=1&interval=10s&timeout=10s")

// 在服务端设置，如果使用默认的节点ip 
s, _ := jsonrpc4go.NewServer("tcp", 3614)
// hostname如果为""，则会自动获取当前节点ip注册
s.SetDiscovery(dc, "127.0.0.1")
s.Register(new(IntRpc))
s.Start()

// 在客户端设置
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
```
### Nacos
```go
dc, _ := nacos.NewNacos("http://127.0.0.1:8849")

// 在服务端设置，如果使用默认的节点ip 
s, _ := jsonrpc4go.NewServer("tcp", 3616)
// hostname如果为""，则会自动获取当前节点ip注册
s.SetDiscovery(dc, "127.0.0.1")
s.Register(new(IntRpc))
s.Start()

// 在客户端设置
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
```

## 📄 License
`jsonrpc4go`代码遵守[Apache-2.0 license](/LICENSE)开源协议。

