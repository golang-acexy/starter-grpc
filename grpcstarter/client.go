package grpcstarter

import (
	"context"

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
func NewClientConn(target string, traceIdSupplier TraceIdSupplier, opts ...grpc.DialOption) (*GrpcClient, error) {
	if traceIdSupplier != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(clientTraceInterceptor(traceIdSupplier)))
	}
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return &GrpcClient{
		gRpcRawClientCoon: conn,
	}, nil
}

func clientTraceInterceptor(traceIdSupplier TraceIdSupplier) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}
		if traceIdSupplier != nil {
			traceId := traceIdSupplier.GetTraceId()
			if traceId != "" {
				md.Set(traceIdKey, traceId)
			}
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// NewClientConnWithResolver 使用resolver配置服务端 创建客户端连接
func NewClientConnWithResolver(target string, traceIdSupplier TraceIdSupplier, iResolver resolver.IResolver, opts ...grpc.DialOption) (*GrpcClient, error) {
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
	return NewClientConn(target, traceIdSupplier, opts...)
}
