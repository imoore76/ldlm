// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.1
// source: ldlm.proto

package protos

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	LDLM_Lock_FullMethodName    = "/ldlm.LDLM/Lock"
	LDLM_TryLock_FullMethodName = "/ldlm.LDLM/TryLock"
	LDLM_Unlock_FullMethodName  = "/ldlm.LDLM/Unlock"
	LDLM_Renew_FullMethodName   = "/ldlm.LDLM/Renew"
)

// LDLMClient is the client API for LDLM service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type LDLMClient interface {
	Lock(ctx context.Context, in *LockRequest, opts ...grpc.CallOption) (*LockResponse, error)
	TryLock(ctx context.Context, in *TryLockRequest, opts ...grpc.CallOption) (*LockResponse, error)
	Unlock(ctx context.Context, in *UnlockRequest, opts ...grpc.CallOption) (*UnlockResponse, error)
	Renew(ctx context.Context, in *RenewRequest, opts ...grpc.CallOption) (*LockResponse, error)
}

type lDLMClient struct {
	cc grpc.ClientConnInterface
}

func NewLDLMClient(cc grpc.ClientConnInterface) LDLMClient {
	return &lDLMClient{cc}
}

func (c *lDLMClient) Lock(ctx context.Context, in *LockRequest, opts ...grpc.CallOption) (*LockResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LockResponse)
	err := c.cc.Invoke(ctx, LDLM_Lock_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lDLMClient) TryLock(ctx context.Context, in *TryLockRequest, opts ...grpc.CallOption) (*LockResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LockResponse)
	err := c.cc.Invoke(ctx, LDLM_TryLock_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lDLMClient) Unlock(ctx context.Context, in *UnlockRequest, opts ...grpc.CallOption) (*UnlockResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UnlockResponse)
	err := c.cc.Invoke(ctx, LDLM_Unlock_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *lDLMClient) Renew(ctx context.Context, in *RenewRequest, opts ...grpc.CallOption) (*LockResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(LockResponse)
	err := c.cc.Invoke(ctx, LDLM_Renew_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LDLMServer is the server API for LDLM service.
// All implementations must embed UnimplementedLDLMServer
// for forward compatibility.
type LDLMServer interface {
	Lock(context.Context, *LockRequest) (*LockResponse, error)
	TryLock(context.Context, *TryLockRequest) (*LockResponse, error)
	Unlock(context.Context, *UnlockRequest) (*UnlockResponse, error)
	Renew(context.Context, *RenewRequest) (*LockResponse, error)
	mustEmbedUnimplementedLDLMServer()
}

// UnimplementedLDLMServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedLDLMServer struct{}

func (UnimplementedLDLMServer) Lock(context.Context, *LockRequest) (*LockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Lock not implemented")
}
func (UnimplementedLDLMServer) TryLock(context.Context, *TryLockRequest) (*LockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TryLock not implemented")
}
func (UnimplementedLDLMServer) Unlock(context.Context, *UnlockRequest) (*UnlockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Unlock not implemented")
}
func (UnimplementedLDLMServer) Renew(context.Context, *RenewRequest) (*LockResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Renew not implemented")
}
func (UnimplementedLDLMServer) mustEmbedUnimplementedLDLMServer() {}
func (UnimplementedLDLMServer) testEmbeddedByValue()              {}

// UnsafeLDLMServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LDLMServer will
// result in compilation errors.
type UnsafeLDLMServer interface {
	mustEmbedUnimplementedLDLMServer()
}

func RegisterLDLMServer(s grpc.ServiceRegistrar, srv LDLMServer) {
	// If the following call pancis, it indicates UnimplementedLDLMServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&LDLM_ServiceDesc, srv)
}

func _LDLM_Lock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LDLMServer).Lock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LDLM_Lock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LDLMServer).Lock(ctx, req.(*LockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LDLM_TryLock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TryLockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LDLMServer).TryLock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LDLM_TryLock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LDLMServer).TryLock(ctx, req.(*TryLockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LDLM_Unlock_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UnlockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LDLMServer).Unlock(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LDLM_Unlock_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LDLMServer).Unlock(ctx, req.(*UnlockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LDLM_Renew_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RenewRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LDLMServer).Renew(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: LDLM_Renew_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LDLMServer).Renew(ctx, req.(*RenewRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// LDLM_ServiceDesc is the grpc.ServiceDesc for LDLM service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LDLM_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "ldlm.LDLM",
	HandlerType: (*LDLMServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Lock",
			Handler:    _LDLM_Lock_Handler,
		},
		{
			MethodName: "TryLock",
			Handler:    _LDLM_TryLock_Handler,
		},
		{
			MethodName: "Unlock",
			Handler:    _LDLM_Unlock_Handler,
		},
		{
			MethodName: "Renew",
			Handler:    _LDLM_Renew_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "ldlm.proto",
}
