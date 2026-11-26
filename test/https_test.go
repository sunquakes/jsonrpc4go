package test

import (
	"testing"

	"github.com/sunquakes/jsonrpc4go/v2"
	"github.com/sunquakes/jsonrpc4go/v2/client"
	"github.com/sunquakes/jsonrpc4go/v2/server"
)

func TestHttpsCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("https", 3621)
	s.Register(new(IntRpc))
	go func() {
		s.SetOptions(server.HttpOptions{KeyPath: "./secure/localhost+2-key.pem", CertPath: "./secure/localhost+2.pem"})
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "https", "127.0.0.1:3621")
	c.SetOptions(&client.HttpOptions{CaPath: "./secure/rootCA.pem"})
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}
