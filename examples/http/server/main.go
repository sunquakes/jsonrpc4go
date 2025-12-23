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
	s, _ := jsonrpc4go.NewServer("http", 3232) // the protocol is http
	// s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3232") // the protocol is tcp
	// s.SetOptions(server.TcpOptions{"aaaaaa", 2 * 1024 * 1024}) // Custom package EOF when the protocol is tcp
	s.SetBeforeFunc(func(id any, method string, params any) error {
		// If the function returns an error, the program stops execution and returns an error message to the client
		// return errors.New("Custom Error")
		return nil
	})
	s.SetAfterFunc(func(id any, method string, result any) error {
		// If the function returns an error, the program stops execution and returns an error message to the client
		// return errors.New("Custom Error")
		return nil
	})
	s.Register(new(IntRpc))
	s.Start()
}
