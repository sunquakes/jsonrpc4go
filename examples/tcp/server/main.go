package main

import "github.com/sunquakes/jsonrpc4go/v2"

type IntRpc struct{}

type Result struct {
	C int `json:"c"`
}

func (i *IntRpc) Add(a, b int) int {
	return a + b
}

func (i *IntRpc) Add2(a, b int) Result {
	return Result{C: a + b}
}

func main() {
	s, _ := jsonrpc4go.NewServer("tcp", 3232)
	s.Register(new(IntRpc))
	s.Start()
}
