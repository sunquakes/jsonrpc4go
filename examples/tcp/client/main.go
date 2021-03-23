package main

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go"
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
	c, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3232")
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
