package grpcstarter

import (
	"context"

	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NewClientConn 创建客户端连接
func NewClientConn(target string, opts ...grpc.DialOption) (*GrpcClient, error) {
	return NewClientConnWithTraceSupplier(target, nil, opts...)
}

// NewClientConnWithTraceSupplier 创建客户端连接
func NewClientConnWithTraceSupplier(target string, traceIdSupplier TraceIdSupplier, opts ...grpc.DialOption) (*GrpcClient, error) {
	if traceIdSupplier != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(clientTraceInterceptor(traceIdSupplier)))
	}
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return &GrpcClient{
		grpcRawClientConn: conn,
	}, nil
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

// NewClientConnWithResolverTraceSupplier 使用resolver配置服务端 创建客户端连接
func NewClientConnWithResolverTraceSupplier(target string, traceIdSupplier TraceIdSupplier, iResolver resolver.IResolver, opts ...grpc.DialOption) (*GrpcClient, error) {
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
	return NewClientConnWithTraceSupplier(target, traceIdSupplier, opts...)
}

func clientTraceInterceptor(traceIdSupplier TraceIdSupplier) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		metadataCtx, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			metadataCtx = metadata.New(nil)
		} else {
			metadataCtx = metadataCtx.Copy()
		}
		if traceIdSupplier != nil {
			traceId := traceIdSupplier.GetTraceId()
			if traceId != "" {
				metadataCtx.Set(traceIdKey, traceId)
			}
		}
		ctx = metadata.NewOutgoingContext(ctx, metadataCtx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

type GrpcClient struct {
	grpcRawClientConn *grpc.ClientConn
}

func (g *GrpcClient) GetRawConn() *grpc.ClientConn {
	return g.grpcRawClientConn
}

func (g *GrpcClient) CloseConn() error {
	return g.grpcRawClientConn.Close()
}
