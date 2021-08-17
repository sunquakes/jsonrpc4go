# jsonrpc4go
## Installing
```
go get -u github.com/sunquakes/jsonrpc4go
```
## Getting started
- Server
```go
package main

import (
    "github.com/sunquakes/jsonrpc4go"
    // "github.com/sunquakes/jsonrpc4go/server"// Custom package EOF when the protocol is tcp
)

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result = int

type Result2 struct {
	C int `json:"c"`
}

func (i *IntRpc) Add(params *Params, result *Result) error {
	a := params.A + params.B
	*result = interface{}(a).(Result)
	return nil
}

func (i *IntRpc) Add2(params *Params, result *Result2) error {
	result.C = params.A + params.B
	return nil
}

func main() {
	s, _ := jsonrpc4go.NewServer("http", "127.0.0.1", "3232") // the protocol is http
	// s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3232") // the protocol is tcp
	// s.SetOptions(server.TcpOptions{"aaaaaa", 2 * 1024 * 1024}) // Custom package EOF when the protocol is tcp
	// s.SetRateLimit(20, 10) //The maximum concurrent number is 10, The maximum request speed is 20 times per second
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
	// "github.com/sunquakes/jsonrpc4go/client" // Custom package EOF when the protocol is tcp
)

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result = int

type Result2 struct {
	C int `json:"c"`
}

func main() {
	result1 := new(Result)
	c, _ := jsonrpc4go.NewClient("http", "127.0.0.1", "3232") // the protocol is http
	// c, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3232") // the protocol is tcp
	// c.SetOptions(client.TcpOptions{"aaaaaa", 2 * 1024 * 1024}) // Custom package EOF when the protocol is tcp
	err1 := c.Call("IntRpc/Add", Params{1, 6}, result1, false) // or "int_rpc/Add", "int_rpc.Add", "IntRpc.Add"
	// data sent: {"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add","params":{"a":1,"b":6}}
	// data received: {"id":"1604283212","jsonrpc":"2.0","result":7}
	fmt.Println(err1) // nil
	fmt.Println(*result1) // 7

	// notify
	result2 := new(Result2)
	err2 := c.Call("int_rpc/Add2", Params{1, 6}, result2, true) // or "IntRpc/Add2", "int_rpc.Add2", "IntRpc.Add2"
	// data sent: {"jsonrpc":"2.0","method":"IntRpc/Add2","params":{"a":1,"b":6}}
	// data received: {"jsonrpc":"2.0","result":{"c":7}}
	fmt.Println(err2) // nil
	fmt.Println(*result2) // {7}

	// batch call
	result3 := new(Result)
	err3 := c.BatchAppend("IntRpc/Add1", Params{1, 6}, result3, false)
	result4 := new(Result)
	err4 := c.BatchAppend("IntRpc/Add", Params{2, 3}, result4, false)
	c.BatchCall()
	// data sent: [{"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add1","params":{"a":1,"b":6}},{"id":"1604283212","jsonrpc":"2.0","method":"IntRpc/Add","params":{"a":2,"b":3}}]
	// data received: [{"id":"1604283212","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}},{"id":"1604283212","jsonrpc":"2.0","result":5}]
	fmt.Println((*err3).Error()) // Method not found
	fmt.Println(*result3) // 0
	fmt.Println(*err4) // nil
	fmt.Println(*result4) // 5
}
```
## Test
```
go test -v ./test/...
```
