package grpcstarter

import (
	"context"
	"github.com/acexy/golang-toolkit/sys"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type GrpcClient struct {
	gRpcRawClientCoon *grpc.ClientConn
}

func (g *GrpcClient) GetConn() *grpc.ClientConn {
	return g.gRpcRawClientCoon
}

func (g *GrpcClient) CloseConn() error {
	return g.gRpcRawClientCoon.Close()
}

// NewClientConn 创建客户端连接
func NewClientConn(target string, opts ...grpc.DialOption) (*GrpcClient, error) {
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return &GrpcClient{
		gRpcRawClientCoon: conn,
	}, nil
}

func ClientTraceInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if !sys.IsEnabledLocalTraceId() {
			return nil
		}
		traceId := sys.GetLocalTraceId()
		if traceId != "" {
			return nil
		}
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}
		md.Set(traceIdKey, sys.GetLocalTraceId())
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewClientConnWithResolver 使用resolver配置服务端 创建客户端连接
func NewClientConnWithResolver(target string, iResolver resolver.IResolver, opts ...grpc.DialOption) (*GrpcClient, error) {
	gResolver, err := iResolver.NewResolver()
	if err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		opts = make([]grpc.DialOption, 1)
		opts[0] = grpc.WithResolvers(gResolver)
	} else {
		opts = append(opts, grpc.WithResolvers(gResolver))
	}
	return NewClientConn(target, opts...)
}
