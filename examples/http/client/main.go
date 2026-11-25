package main

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go"
)

type Result struct {
	C int `json:"c"`
}

func main() {
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3232") // or "IntRpc/Add2", "int_rpc.Add2", "IntRpc.Add2"
	result1, err1 := jsonrpc4go.Call[int](c, "Add", 1, 6)            // or "int_rpc/Add", "int_rpc.Add", "IntRpc.Add"
	// data sent: {"id":"1","jsonrpc":"2.0","method":"IntRpc/Add","params":[1,6]}
	// data received: {"id":"1","jsonrpc":"2.0","result":7}
	fmt.Println(err1)    // nil
	fmt.Println(result1) // 7

	// notify
	err2 := c.Notify("Add2", 1, 6)
	// data sent: {"jsonrpc":"2.0","method":"IntRpc/Add2","params":[1,6]}
	fmt.Println(err2) // nil

	// bind a method once, then call it like a local function
	add := jsonrpc4go.Bind[int](c, "Add")
	add2 := jsonrpc4go.Bind[Result](c, "Add2")
	b1, err3 := add(1, 6)
	b2, err4 := add2(2, 3)
	fmt.Println(b1, err3)   // 7 nil
	fmt.Println(b2.C, err4) // 5 nil

	// batch call
	c.BatchAppend("Add1", 1, 6)
	c.BatchAppend("Add", 2, 3)
	results, err := c.BatchCall()
	// data sent: [{"id":"1","jsonrpc":"2.0","method":"IntRpc/Add1","params":[1,6]},{"id":"2","jsonrpc":"2.0","method":"IntRpc/Add","params":[2,3]}]
	// data received: [{"id":"1","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}},{"id":"2","jsonrpc":"2.0","result":5}]
	fmt.Println(err)               // <nil>
	fmt.Println(results[0].Err)    // Method not found
	fmt.Println(results[1].Result) // 5
}
