package main

import "github.com/sunquakes/jsonrpc4go"

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
	s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3232")
	s.Register(new(IntRpc))
	s.Start()
}
