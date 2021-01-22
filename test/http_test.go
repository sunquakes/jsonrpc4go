package test

import (
	go_jsonrpc "github.com/iloveswift/go-jsonrpc"
	"github.com/iloveswift/go-jsonrpc/common"
	"testing"
	"time"
)

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result = int

func (i *IntRpc) Add(params *Params, result *Result) error {
	a := params.A + params.B
	*result = interface{}(a).(Result)
	return nil
}

func TestHttpCall(t *testing.T) {
	go func() {
		s, _ := go_jsonrpc.NewServer("http", "127.0.0.1", "3232")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := go_jsonrpc.NewClient("http", "127.0.0.1", "3232")
	params := Params{1, 2}
	result := new(Result)
	c.Call("IntRpc.Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestHttpCallMethod(t *testing.T) {
	go func() {
		s, _ := go_jsonrpc.NewServer("http", "127.0.0.1", "3238")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := go_jsonrpc.NewClient("http", "127.0.0.1", "3238")
	params := Params{1, 2}
	result := new(Result)
	c.Call("int_rpc/Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestHttpNotifyCall(t *testing.T) {
	go func() {
		s, _ := go_jsonrpc.NewServer("http", "127.0.0.1", "3233")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := go_jsonrpc.NewClient("http", "127.0.0.1", "3233")
	params := Params{2, 3}
	result := new(Result)
	c.Call("IntRpc.Add", &params, result, true)
	if *result != 5 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 5, *result)
	}
}

func TestHttpBatchCall(t *testing.T) {
	go func() {
		s, _ := go_jsonrpc.NewServer("http", "127.0.0.1", "3236")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := go_jsonrpc.NewClient("http", "127.0.0.1", "3236")

	result1 := new(Result)
	err1 := c.BatchAppend("IntRpc/Add1", Params{1, 6}, result1, false)
	result2 := new(Result)
	err2 := c.BatchAppend("IntRpc/Add", Params{2, 3}, result2, false)
	c.BatchCall()

	if *err2 != nil || *result2 != 5 {
		t.Errorf("%d + %d expected be %d, but %d got", 2, 3, 5, result2)
	}
	if (*err1).Error() != common.CodeMap[common.MethodNotFound] {
		t.Errorf("Error message expected be %s, but %s got", common.CodeMap[common.MethodNotFound], (*err1).Error())
	}
}
