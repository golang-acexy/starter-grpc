package grpcstarter

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type regressionTraceSupplier struct {
	order *[]string
}

func (*regressionTraceSupplier) SetTraceId(string) {}

func (s *regressionTraceSupplier) GetTraceId() string {
	*s.order = append(*s.order, "trace")
	return "regression-trace"
}

func TestTracePreservesCallerInterceptorAndMetadata(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if !reflect.DeepEqual(md.Get(traceIdKey), []string{"regression-trace"}) || !reflect.DeepEqual(md.Get("custom"), []string{"retained"}) {
			t.Errorf("unexpected RPC metadata: %v", md)
		}
		return handler(ctx, req)
	}))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })

	var order []string
	client, err := NewClientConnWithTraceSupplier("passthrough:///regression", &regressionTraceSupplier{order: &order},
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			order = append(order, "caller")
			return invoker(ctx, method, req, reply, cc, opts...)
		}),
		grpc.WithChainUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			order = append(order, "chain")
			return invoker(ctx, method, req, reply, cc, opts...)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.CloseConn() })
	original := metadata.Pairs(traceIdKey, "original", "custom", "retained")
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), original), 5*time.Second)
	defer cancel()
	if _, err := healthpb.NewHealthClient(client.GetRawConn()).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(order, []string{"caller", "chain", "trace"}) {
		t.Fatalf("caller interceptor was lost or order changed: %v", order)
	}
	if !reflect.DeepEqual(original.Get(traceIdKey), []string{"original"}) {
		t.Fatal("trace injection mutated caller metadata")
	}
}
