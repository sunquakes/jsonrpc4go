// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.24.3
// source: discovery/etcd/etcdserverpb/lease.proto

package etcdserverpb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// LeaseClient is the client API for Lease service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type LeaseClient interface {
	LeaseGrant(ctx context.Context, in *LeaseGrantRequest, opts ...grpc.CallOption) (*LeaseGrantResponse, error)
	LeaseKeepAlive(ctx context.Context, in *LeaseKeepAliveRequest, opts ...grpc.CallOption) (*LeaseKeepAliveResponse, error)
}

type leaseClient struct {
	cc grpc.ClientConnInterface
}

func NewLeaseClient(cc grpc.ClientConnInterface) LeaseClient {
	return &leaseClient{cc}
}

func (c *leaseClient) LeaseGrant(ctx context.Context, in *LeaseGrantRequest, opts ...grpc.CallOption) (*LeaseGrantResponse, error) {
	out := new(LeaseGrantResponse)
	err := c.cc.Invoke(ctx, "/etcdserverpb.Lease/LeaseGrant", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *leaseClient) LeaseKeepAlive(ctx context.Context, in *LeaseKeepAliveRequest, opts ...grpc.CallOption) (*LeaseKeepAliveResponse, error) {
	out := new(LeaseKeepAliveResponse)
	err := c.cc.Invoke(ctx, "/etcdserverpb.Lease/LeaseKeepAlive", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LeaseServer is the server API for Lease service.
// All implementations must embed UnimplementedLeaseServer
// for forward compatibility
type LeaseServer interface {
	LeaseGrant(context.Context, *LeaseGrantRequest) (*LeaseGrantResponse, error)
	LeaseKeepAlive(context.Context, *LeaseKeepAliveRequest) (*LeaseKeepAliveResponse, error)
	mustEmbedUnimplementedLeaseServer()
}

// UnimplementedLeaseServer must be embedded to have forward compatible implementations.
type UnimplementedLeaseServer struct {
}

func (UnimplementedLeaseServer) LeaseGrant(context.Context, *LeaseGrantRequest) (*LeaseGrantResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LeaseGrant not implemented")
}
func (UnimplementedLeaseServer) LeaseKeepAlive(context.Context, *LeaseKeepAliveRequest) (*LeaseKeepAliveResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LeaseKeepAlive not implemented")
}
func (UnimplementedLeaseServer) mustEmbedUnimplementedLeaseServer() {}

// UnsafeLeaseServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LeaseServer will
// result in compilation errors.
type UnsafeLeaseServer interface {
	mustEmbedUnimplementedLeaseServer()
}

func RegisterLeaseServer(s grpc.ServiceRegistrar, srv LeaseServer) {
	s.RegisterService(&Lease_ServiceDesc, srv)
}

func _Lease_LeaseGrant_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LeaseGrantRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LeaseServer).LeaseGrant(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/etcdserverpb.Lease/LeaseGrant",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LeaseServer).LeaseGrant(ctx, req.(*LeaseGrantRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Lease_LeaseKeepAlive_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LeaseKeepAliveRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LeaseServer).LeaseKeepAlive(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/etcdserverpb.Lease/LeaseKeepAlive",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LeaseServer).LeaseKeepAlive(ctx, req.(*LeaseKeepAliveRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Lease_ServiceDesc is the grpc.ServiceDesc for Lease service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Lease_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "etcdserverpb.Lease",
	HandlerType: (*LeaseServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "LeaseGrant",
			Handler:    _Lease_LeaseGrant_Handler,
		},
		{
			MethodName: "LeaseKeepAlive",
			Handler:    _Lease_LeaseKeepAlive_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "discovery/etcd/etcdserverpb/lease.proto",
}
