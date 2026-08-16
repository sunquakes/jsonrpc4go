[English](README.md) | 🇨🇳中文
# jsonrpc4go

> 一个高性能的 [JSON-RPC 2.0](https://www.jsonrpc.org/specification) Go 语言实现。支持 HTTP/TCP、服务注册与发现（Consul / Nacos / etcd）、客户端负载均衡、限流、钩子、批量请求与通知。

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

## 🧰 安装
```
go get -u github.com/sunquakes/jsonrpc4go
```
## 📖 开始使用
- 服务端代码
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

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result2 struct {
	C int `json:"c"`
}

func main() {
	result := new(int)
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3232") // http协议
	err := c.Call("Add", Params{1, 6}, result, false)
	// 发送的数据格式: {"id":"1604283212", "jsonrpc":"2.0", "method":"IntRpc/Add", "params":{"a":1,"b":6}}
	// 接收的数据格式: {"id":"1604283212", "jsonrpc":"2.0", "result":7}
	fmt.Println(err) // nil
	fmt.Println(*result) // 7
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
s.SetBeforeFunc(func(id interface{}, method string, params interface{}) error {
    // 如果方法返回error类型，服务端停止执行并返回错误信息到客户端
    // 例：return errors.New("Custom Error")
    return nil
})
// 在方法后执行的钩子方法
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
// 通知
result2 := new(Result2)
err2 := c.Call("Add2", Params{1, 6}, result2, true)
// 发送的数据格式: {"jsonrpc":"2.0","method":"IntRpc/Add2","params":{"a":1,"b":6}}
// 接收的数据格式: {"jsonrpc":"2.0","result":{"c":7}}
fmt.Println(err2) // nil
fmt.Println(*result2) // {7}
```
- 批量请求
```go
// 批量请求
result3 := new(int)
err3 := c.BatchAppend("Add1", Params{1, 6}, result3, false)
result4 := new(int)
err4 := c.BatchAppend("Add", Params{2, 3}, result4, false)
c.BatchCall()
// 发送的数据格式: [{"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add1","params":{"a":1,"b":6}},{"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add","params":{"a":2,"b":3}}]
// 接收的数据格式: [{"id":"1604283212","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}},{"id":"1604283212","jsonrpc":"2.0","result":5}]
fmt.Println((*err3).Error()) // Method not found
fmt.Println(*result3) // 0
fmt.Println(*err4) // nil
fmt.Println(*result4) // 5
```
- 用户端负载均衡
```go
c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3232,127.0.0.1:3233,127.0.0.1:3234")
```

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

