package test

import (
	"context"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery/etcd/etcdserverpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

type KVInterface interface {
	Put(context.Context, *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error)
}

type MockKVService struct{}

func (s *MockKVService) Put(ctx context.Context, data *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	return &etcdserverpb.PutResponse{}, nil
}

var MockKVServiceDesc = grpc.ServiceDesc{
	ServiceName: "etcdserverpb.KV", // Full service name
	HandlerType: (*KVInterface)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Put",
			Handler:    _MockMockKVServiceService_Put_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "kv.proto",
}

func _MockMockKVServiceService_Put_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	// Implement your Put method handler here
	return nil, nil
}

func TestRegister(t *testing.T) {
	bufListener := bufconn.Listen(1024 * 1024)

	// Create a gRPC server
	grpcServer := grpc.NewServer()
	grpcServer.RegisterService(&MockKVServiceDesc, &MockKVService{})
	// Start the server in a goroutine
	go func() {
		if err := grpcServer.Serve(bufListener); err != nil {
			t.Fatalf("gRPC server error: %v", err)
		}
	}()
	defer grpcServer.Stop()

	// Set up a client connection to the gRPC server using bufconn.Dialer
	conn, err := bufListener.Dial()
	if err != nil {
		t.Fatalf("Failed to dial bufconn: %v", err)
	}
	defer conn.Close()

	// Create a gRPC client connection using grpc.Dial
	clientConn, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
		return conn, nil
	}))
	if err != nil {
		t.Fatalf("Failed to create gRPC client connection: %v", err)
	}
	defer clientConn.Close()

	// Create a gRPC client using the client connection
	client := etcdserverpb.NewKVClient(clientConn)

	// Make a gRPC call using the client
	request := &etcdserverpb.PutRequest{} // Provide your request data
	response, err := client.Put(context.Background(), request)
	if err != nil {
		t.Fatalf("gRPC call error: %v", err)
	}
	fmt.Print(response)
}
