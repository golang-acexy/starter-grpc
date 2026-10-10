package grpcstarter

import (
	"context"

	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// NewClientConn creates a caller-owned connection; callers must supply transport credentials.
// Creation is lazy and does not establish remote readiness; use RPC contexts for deadlines.
func NewClientConn(target string, opts ...grpc.DialOption) (*GrpcClient, error) {
	return NewClientConnWithTraceSupplier(target, nil, opts...)
}

// NewClientConnWithTraceSupplier adds unary trace propagation without replacing caller interceptors.
// The supplier must isolate concurrent RPC state; nil disables tracing. Callers own connection closure.
func NewClientConnWithTraceSupplier(target string, traceIdSupplier TraceIdSupplier, opts ...grpc.DialOption) (*GrpcClient, error) {
	if traceIdSupplier != nil {
		opts = append(opts, grpc.WithChainUnaryInterceptor(clientTraceInterceptor(traceIdSupplier)))
	}
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return &GrpcClient{
		grpcRawClientConn: conn,
	}, nil
}

// NewClientConnWithResolver creates a caller-owned connection using a nonnil resolver provider.
// The target scheme must match the builder; callers supply credentials and balancing options.
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

// NewClientConnWithResolverTraceSupplier combines discovery with optional unary trace propagation.
// The resolver provider must be nonnil; callers own connection and discovery-client closure.
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

// GrpcClient wraps a reusable caller-owned gRPC connection created by the client helpers.
// Its zero value is not usable; server starter shutdown does not close client connections.
type GrpcClient struct {
	grpcRawClientConn *grpc.ClientConn
}

// GetRawConn exposes the shared connection for generated clients; no copy is made.
func (g *GrpcClient) GetRawConn() *grpc.ClientConn {
	return g.grpcRawClientConn
}

// CloseConn closes the connection and its resolvers, interrupting outstanding RPCs.
// Caller-owned Nacos and Etcd clients remain open and must be closed separately.
func (g *GrpcClient) CloseConn() error {
	return g.grpcRawClientConn.Close()
}
