package test

import (
	"context"
	"log"
	"net"
	"net/url"
	"testing"

	"github.com/sunquakes/jsonrpc4go/discovery/etcd"
	"github.com/sunquakes/jsonrpc4go/discovery/etcd/etcdserverpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type KVInterface interface {
	Put(context.Context, *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error)
	Range(context.Context, *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error)
}

type MockKVService struct{}

func (s *MockKVService) Put(ctx context.Context, data *etcdserverpb.PutRequest) (*etcdserverpb.PutResponse, error) {
	return &etcdserverpb.PutResponse{}, nil
}

func (s *MockKVService) Range(ctx context.Context, data *etcdserverpb.RangeRequest) (*etcdserverpb.RangeResponse, error) {
	return &etcdserverpb.RangeResponse{}, nil
}

var MockKVServiceDesc = grpc.ServiceDesc{
	ServiceName: "etcdserverpb.KV", // Full service name
	HandlerType: (*KVInterface)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Put",
			Handler:    _MockMockKVServiceService_Put_Handler,
		},
		{
			MethodName: "Range",
			Handler:    _MockMockKVServiceService_Range_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "kv.proto",
}

func _MockMockKVServiceService_Put_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return &etcdserverpb.PutResponse{}, nil
}

func _MockMockKVServiceService_Range_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	keyValue := &etcdserverpb.KeyValue{}
	keyValue.Key = "java_tcp"
	keyValue.Value = []byte("{\"UniqueId\":\"1692416183\",\"Name\":\"java_tcp\",\"Addr\":\"192.168.1.15:3232\"}")
	resp := &etcdserverpb.RangeResponse{}
	resp.Kvs = []*etcdserverpb.KeyValue{keyValue}
	return resp, nil
}

type LeaseInterface interface {
	LeaseGrant(context.Context, *etcdserverpb.LeaseGrantRequest) (*etcdserverpb.LeaseGrantResponse, error)
}

type MockLeaseService struct{}

func (s *MockLeaseService) LeaseGrant(ctx context.Context, data *etcdserverpb.LeaseGrantRequest) (*etcdserverpb.LeaseGrantResponse, error) {
	return &etcdserverpb.LeaseGrantResponse{}, nil
}

var MockLeaseServiceDesc = grpc.ServiceDesc{
	ServiceName: "etcdserverpb.Lease", // Full service name
	HandlerType: (*LeaseInterface)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "LeaseGrant",
			Handler:    _MockMockLeaseServiceService_LeaseGrant_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "lease.proto",
}

func _MockMockLeaseServiceService_LeaseGrant_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return &etcdserverpb.LeaseGrantRequest{}, nil
}

func TestEtcdRegister(t *testing.T) {
	bufListener := bufconn.Listen(1024 * 1024)

	// Create a gRPC server
	grpcServer := grpc.NewServer()
	grpcServer.RegisterService(&MockKVServiceDesc, &MockKVService{})
	grpcServer.RegisterService(&MockLeaseServiceDesc, &MockLeaseService{})
	// Start the server in a goroutine
	go func() {
		if err := grpcServer.Serve(bufListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
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
	clientConn, err := grpc.NewClient("192.168.1.15:3232", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return conn, nil
	}))
	if err != nil {
		t.Fatalf("Failed to create gRPC client connection: %v", err)
	}
	defer clientConn.Close()

	listenerAddr := "bufconn://" + bufListener.Addr().String()
	URL, err := url.Parse(listenerAddr)
	if err != nil {
		t.Error(err)
	}

	heartbeat := make(chan bool)
	r := &etcd.Etcd{URL: URL, Conn: clientConn, Heartbeat: heartbeat}
	// r, err := etcd.NewEtcd("grpc://127.0.0.1:2379")
	if err != nil {
		t.Error(err)
	}
	err = r.Register("java_tcp", "tcp", "192.168.1.15", 3232)
	if err != nil {
		t.Error(err)
	}
}

func TestEtcdGet(t *testing.T) {
	bufListener := bufconn.Listen(1024 * 1024)

	// Create a gRPC server
	grpcServer := grpc.NewServer()
	grpcServer.RegisterService(&MockKVServiceDesc, &MockKVService{})
	// Start the server in a goroutine
	go func() {
		if err := grpcServer.Serve(bufListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
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
	clientConn, err := grpc.NewClient("192.168.1.15:3232", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return conn, nil
	}))
	if err != nil {
		t.Fatalf("Failed to create gRPC client connection: %v", err)
	}
	defer clientConn.Close()

	listenerAddr := "bufconn://" + bufListener.Addr().String()
	URL, err := url.Parse(listenerAddr)
	if err != nil {
		t.Error(err)
	}

	heartbeat := make(chan bool)
	r := &etcd.Etcd{URL: URL, Conn: clientConn, Heartbeat: heartbeat}
	// r, err := etcd.NewEtcd("grpc://127.0.0.1:2379")
	if err != nil {
		t.Error(err)
	}
	servers, err := r.Get("java_tcp")
	if err != nil {
		t.Error(err)
	}
	expected := "192.168.1.15:3232"
	if servers != expected {
		t.Errorf("URL expected be %s, but %s got", expected, servers)
	}
}
