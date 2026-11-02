package grpcstarter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
	"weak"

	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"google.golang.org/grpc"
	channelzpb "google.golang.org/grpc/channelz/grpc_channelz_v1"
	channelzservice "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

type pressureTraceSupplier struct{}

func (pressureTraceSupplier) GetTraceId() string { return "pressure-trace" }
func (pressureTraceSupplier) SetTraceId(string)  {}

func pressureAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	return address
}

func pressureServer(t *testing.T) (*GrpcStarter, string) {
	t.Helper()
	address := pressureAddress(t)
	starter := &GrpcStarter{Config: GrpcConfig{ListenAddress: address, TraceIdSupplier: pressureTraceSupplier{}, RegisterService: func(server *grpc.Server) {
		healthpb.RegisterHealthServer(server, health.NewServer())
		channelzservice.RegisterChannelzServiceToServer(server)
	}}}
	if _, err := starter.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if RawGrpcServer() != nil {
			starter.Stop(time.Second)
		}
	})
	return starter, address
}

func pressureClient(t *testing.T, address string, trace bool) *GrpcClient {
	t.Helper()
	var supplier TraceIdSupplier
	if trace {
		supplier = pressureTraceSupplier{}
	}
	client, err := NewClientConnWithTraceSupplier("passthrough:///"+address, supplier, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.CloseConn() })
	return client
}

func TestGrpcMemoryPressure(t *testing.T) {
	if os.Getenv("GRPC_MEMORY_STRESS") != "1" {
		t.Skip("set GRPC_MEMORY_STRESS=1")
	}
	_, address := pressureServer(t)
	for _, trace := range []bool{false, true} {
		t.Run(fmt.Sprintf("trace=%t", trace), func(t *testing.T) {
			client := pressureClient(t, address, trace)
			stub := healthpb.NewHealthClient(client.GetRawConn())
			run := func(count int) []weak.Pointer[healthpb.HealthCheckResponse] {
				var wg sync.WaitGroup
				var mu sync.Mutex
				refs := make([]weak.Pointer[healthpb.HealthCheckResponse], 0, count)
				for w := 0; w < 16; w++ {
					wg.Add(1)
					go func(w int) {
						defer wg.Done()
						for i := w; i < count; i += 16 {
							ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(context.Background(), metadata.Pairs("custom", "retained")), 3*time.Second)
							reply, err := stub.Check(ctx, &healthpb.HealthCheckRequest{})
							cancel()
							if err != nil || reply.GetStatus() != healthpb.HealthCheckResponse_SERVING {
								t.Error("health RPC failed")
								return
							}
							mu.Lock()
							refs = append(refs, weak.Make(reply))
							mu.Unlock()
						}
					}(w)
				}
				wg.Wait()
				return refs
			}
			run(100)
			runtime.GC()
			runtime.GC()
			var baseline runtime.MemStats
			runtime.ReadMemStats(&baseline)
			baseG := runtime.NumGoroutine()
			for batch := 1; batch <= 5; batch++ {
				start := time.Now()
				refs := run(1000)
				elapsed := time.Since(start)
				runtime.GC()
				runtime.GC()
				live := 0
				for _, ref := range refs {
					if ref.Value() != nil {
						live++
					}
				}
				var mem runtime.MemStats
				runtime.ReadMemStats(&mem)
				t.Logf("batch=%d ops_s=%.0f heap=%d delta=%d allocated=%d live=%d goroutines=%d baseline_goroutines=%d", batch, 1000/elapsed.Seconds(), mem.HeapAlloc, int64(mem.HeapAlloc)-int64(baseline.HeapAlloc), mem.TotalAlloc-baseline.TotalAlloc, live, runtime.NumGoroutine(), baseG)
				if live != 0 || len(refs) != 1000 {
					t.Fatal("responses retained or batch incomplete")
				}
			}
		})
	}
}

func TestGrpcStreamCancelAndForcedStop(t *testing.T) {
	starter, address := pressureServer(t)
	client := pressureClient(t, address, false)
	stub := healthpb.NewHealthClient(client.GetRawConn())
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := stub.Watch(ctx, &healthpb.HealthCheckRequest{})
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if _, err = stream.Recv(); err != nil {
			cancel()
			t.Fatal(err)
		}
		cancel()
		if _, err = stream.Recv(); err == nil {
			t.Fatal("stream ignored cancellation")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := stub.Watch(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Recv(); err != nil {
		t.Fatal(err)
	}
	g, s, err := starter.Stop(time.Millisecond)
	if g || !s || !errors.Is(err, ErrGrpcStopTimeout) {
		t.Fatalf("unexpected stop: %t %t %v", g, s, err)
	}
	if _, err = stream.Recv(); err == nil {
		t.Fatal("forced stop did not interrupt stream")
	}
}

type pressureGrpcRefs struct {
	server weak.Pointer[grpc.Server]
	client weak.Pointer[grpc.ClientConn]
}

func pressureCycle(t *testing.T) pressureGrpcRefs {
	t.Helper()
	starter, address := pressureServer(t)
	client := pressureClient(t, address, false)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := healthpb.NewHealthClient(client.GetRawConn()).Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	refs := pressureGrpcRefs{weak.Make(RawGrpcServer()), weak.Make(client.GetRawConn())}
	client.CloseConn()
	if g, s, err := starter.Stop(time.Second); !g || !s || err != nil {
		t.Fatal("cycle stop failed")
	}
	// Helpers register cleanup closures that intentionally hold pointers until this subtest ends.
	return refs
}
func TestGrpcLifecycleReclaimsConnections(t *testing.T) {
	if os.Getenv("GRPC_MEMORY_STRESS") != "1" {
		t.Skip("set GRPC_MEMORY_STRESS=1")
	}
	refs := make([]pressureGrpcRefs, 0, 30)
	for i := 0; i < 30; i++ {
		t.Run(fmt.Sprintf("cycle%d", i), func(t *testing.T) { refs = append(refs, pressureCycle(t)) })
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()
		runtime.GC()
		live := 0
		for _, ref := range refs {
			if ref.server.Value() != nil || ref.client.Value() != nil {
				live++
			}
		}
		if live == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("retained server/client pairs: %d", live)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestFailedBindChannelzRetention verifies failed listener binding removes diagnostic registrations.
func TestFailedBindChannelzRetention(t *testing.T) {
	_, address := pressureServer(t)
	client := pressureClient(t, address, false)
	probe := channelzpb.NewChannelzClient(client.GetRawConn())
	count := func() int {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		response, err := probe.GetServers(ctx, &channelzpb.GetServersRequest{MaxResults: 10000})
		if err != nil {
			t.Fatal(err)
		}
		return len(response.Server)
	}
	baseline := count()
	// Withdraw the running starter so a second starter can reach the occupied-listener path.
	current := grpcRuntimeState.Swap(nil)
	grpcLifecycleLock.Lock()
	grpcState = grpcStopped
	grpcLifecycleLock.Unlock()
	defer func() {
		grpcRuntimeState.Store(current)
		grpcLifecycleLock.Lock()
		grpcState = grpcRunning
		grpcLifecycleLock.Unlock()
	}()
	var failed []*grpc.Server
	defer func() {
		for _, server := range failed {
			server.Stop()
		}
	}()
	for i := 0; i < 20; i++ {
		starter := &GrpcStarter{Config: GrpcConfig{ListenAddress: address, RegisterService: func(server *grpc.Server) { failed = append(failed, server) }}}
		if _, err := starter.Start(); err == nil {
			t.Fatal("expected address-in-use failure")
		}
	}
	retained := count() - baseline
	t.Logf("failed_starts=20 retained_channelz_servers=%d", retained)
	if retained != 0 {
		t.Fatal("failed listener binding leaked diagnostic entries")
	}
	for _, server := range failed {
		server.Stop()
	}
	if count() != baseline {
		t.Fatal("explicit Stop did not remove diagnostic entries")
	}
}

func TestClientOptionsBackingStorage(t *testing.T) {
	for _, mode := range []string{"trace", "resolver", "resolver_trace"} {
		t.Run(mode, func(t *testing.T) {
			marker := grpc.WithUserAgent("marker")
			backing := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials()), marker, marker, marker}
			provider := resolver.NewStaticResolver([]string{"127.0.0.1:1"})
			create := func() {
				var client *GrpcClient
				var err error
				switch mode {
				case "trace":
					client, err = NewClientConnWithTraceSupplier("passthrough:///unused", pressureTraceSupplier{}, backing[:1]...)
				case "resolver":
					client, err = NewClientConnWithResolver("static:///unused", provider, backing[:1]...)
				case "resolver_trace":
					client, err = NewClientConnWithResolverTraceSupplier("static:///unused", pressureTraceSupplier{}, provider, backing[:1]...)
				}
				if err != nil {
					t.Error(err)
					return
				}
				client.CloseConn()
			}
			create()
			for _, option := range backing[1:] {
				if option != marker {
					t.Fatal("caller option storage overwritten")
				}
			}
			var wg sync.WaitGroup
			for range 16 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for range 10 {
						create()
					}
				}()
			}
			wg.Wait()
			for _, option := range backing[1:] {
				if option != marker {
					t.Fatal("concurrent constructors overwrote caller storage")
				}
			}
		})
	}
}

func BenchmarkTraceMetadata(b *testing.B) {
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("a", "1", "b", "2", "c", "3"))
	interceptor := clientTraceInterceptor(pressureTraceSupplier{})
	invoke := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error { return nil }
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := interceptor(ctx, "/test", nil, nil, nil, invoke); err != nil {
			b.Fatal(err)
		}
	}
}
