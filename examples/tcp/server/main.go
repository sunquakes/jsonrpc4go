package main

import "github.com/sunquakes/jsonrpc4go"

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result struct {
	C int `json:"c"`
}

func (i *IntRpc) Add(params *Params, result *int) error {
	*result = params.A + params.B
	return nil
}

func (i *IntRpc) Add2(params *Params, result *Result) error {
	result.C = params.A + params.B
	return nil
}

func main() {
	s, _ := jsonrpc4go.NewServer("tcp", 3232)
	s.Register(new(IntRpc))
	s.Start()
}
