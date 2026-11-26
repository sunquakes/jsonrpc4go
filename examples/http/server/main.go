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
	s, _ := jsonrpc4go.NewServer("http", 3232) // the protocol is http
	// s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3232") // the protocol is tcp
	// s.SetOptions(server.TcpOptions{"aaaaaa", 2 * 1024 * 1024}) // Custom package EOF when the protocol is tcp
	s.SetBeforeFunc(func(id any, method string, params any) error {
		// params is a []any of the decoded positional arguments, e.g. []any{1, 6}
		// If the function returns an error, the program stops execution and returns an error message to the client
		// return errors.New("Custom Error")
		return nil
	})
	s.SetAfterFunc(func(id any, method string, result any) error {
		// result is the value returned by the method, e.g. 7
		// If the function returns an error, the program stops execution and returns an error message to the client
		// return errors.New("Custom Error")
		return nil
	})
	s.Register(new(IntRpc))
	s.Start()
}
