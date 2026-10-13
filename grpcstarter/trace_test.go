package grpcstarter

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type traceTestSupplier struct {
	id  string
	set chan string
}

// GetTraceId returns the outgoing test ID.
func (s *traceTestSupplier) GetTraceId() string { return s.id }

// SetTraceId records the ID received by the server interceptor.
func (s *traceTestSupplier) SetTraceId(id string) { s.set <- id }

type traceTestHealthServer struct {
	healthpb.UnimplementedHealthServer
	check func(context.Context)
}

// Check observes trace propagation inside the business handler.
func (s *traceTestHealthServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	s.check(ctx)
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func TestTraceIDClientToServer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clientID string
		disable  bool
		incoming []string
		want     []string
	}{
		{name: "propagate supplier ID", clientID: "client-trace", want: []string{"client-trace"}},
		{name: "replace existing IDs", clientID: "new-trace", incoming: []string{"old", "older"}, want: []string{"new-trace"}},
		{name: "empty supplier preserves metadata", incoming: []string{"existing"}, want: []string{"existing"}},
		{name: "missing ID skips setter"},
		{name: "disabled tracing preserves metadata and server uses first ID", disable: true, incoming: []string{"first", "second"}, want: []string{"first", "second"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			supplier := &traceTestSupplier{set: make(chan string, 4)}
			listener := bufconn.Listen(1024 * 1024)
			server := grpc.NewServer(grpc.UnaryInterceptor(serverTraceInterceptor(supplier)))
			healthpb.RegisterHealthServer(server, &traceTestHealthServer{check: func(ctx context.Context) {
				md, _ := metadata.FromIncomingContext(ctx)
				if got := md.Get(traceIdKey); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("incoming IDs = %v, want %v", got, tc.want)
				}
				if got := md.Get("custom"); !reflect.DeepEqual(got, []string{"retained"}) {
					t.Errorf("custom metadata = %v, want [retained]", got)
				}
				select {
				case got := <-supplier.set:
					if len(tc.want) == 0 {
						t.Errorf("unexpected SetTraceId(%q)", got)
					} else if got != tc.want[0] {
						t.Errorf("SetTraceId received %q, want %q", got, tc.want[0])
					}
				default:
					if len(tc.want) > 0 {
						t.Error("SetTraceId was not called before business handler")
					}
				}
			}})
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(func() { server.Stop(); _ = listener.Close() })
			var clientSupplier TraceIdSupplier
			if !tc.disable {
				clientSupplier = &traceTestSupplier{id: tc.clientID}
			}
			client, err := NewClientConnWithTraceSupplier("passthrough:///trace-test", clientSupplier,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = client.CloseConn() })
			original := metadata.Pairs("custom", "retained")
			if len(tc.incoming) > 0 {
				original.Set(traceIdKey, tc.incoming...)
			}
			before := original.Copy()
			ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), original), 5*time.Second)
			defer cancel()
			response, err := healthpb.NewHealthClient(client.GetRawConn()).Check(ctx, &healthpb.HealthCheckRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
				t.Fatalf("unexpected health response: %v", response)
			}
			if !reflect.DeepEqual(original, before) {
				t.Errorf("caller metadata mutated: got %v, want %v", original, before)
			}
			select {
			case got := <-supplier.set:
				t.Errorf("unexpected additional SetTraceId(%q)", got)
			default:
			}
		})
	}
}
