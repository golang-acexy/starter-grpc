package grpcstarter

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-acexy/starter-parent/parent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const traceIdKey = "trace-id"

var grpcRuntimeState atomic.Pointer[grpcRuntime]
var grpcLifecycleLock sync.Mutex
var grpcState grpcLifecycleState

type grpcRuntime struct {
	server *grpc.Server
	done   <-chan struct{}
}

type grpcLifecycleState uint8

const (
	grpcStopped grpcLifecycleState = iota
	grpcStarting
	grpcRunning
	grpcStopping
)

// TraceIdSupplier bridges unary trace metadata to application-owned trace storage.
// Implementations must isolate concurrent RPCs and manage cleanup; streaming RPCs are not traced.
type TraceIdSupplier interface {
	// SetTraceId receives the first incoming trace-id; missing IDs do not clear existing state.
	SetTraceId(traceId string)
	// GetTraceId returns the outgoing trace ID; empty leaves existing outgoing metadata unchanged.
	GetTraceId() string
}

// GrpcConfig configures the process-wide server and unary trace integration.
type GrpcConfig struct {
	// Network defaults to tcp when empty.
	Network string
	// ListenAddress defaults to :8081 when empty.
	ListenAddress string
	// InitFunc is invoked by the default parent post-start callback, not by direct Start calls.
	InitFunc func(instance *grpc.Server)
	// TraceIdSupplier enables unary tracing when nonnil.
	TraceIdSupplier TraceIdSupplier
	// RegisterService registers protobuf services synchronously before the listener is bound.
	RegisterService func(g *grpc.Server)
}

// GrpcStarter manages one process-wide gRPC server under the parent lifecycle.
// Configuration is resolved once; callers should sequence normal startup and shutdown.
type GrpcStarter struct {
	// Config is used when LazyConfig is nil.
	Config GrpcConfig
	// LazyConfig takes precedence and is evaluated at most once.
	LazyConfig func() GrpcConfig
	config     *GrpcConfig
	configOnce sync.Once
	// GrpcSetting overrides defaults, including the callback that invokes InitFunc.
	GrpcSetting *parent.Setting
}

func (g *GrpcStarter) getConfig() *GrpcConfig {
	g.configOnce.Do(func() {
		var config GrpcConfig
		if g.LazyConfig != nil {
			config = g.LazyConfig()
		} else {
			config = g.Config
		}
		if config.Network == "" {
			config.Network = "tcp"
		}
		if config.ListenAddress == "" {
			config.ListenAddress = ":8081"
		}
		g.config = &config
	})
	return g.config
}

// Setting returns an override or no-restart defaults with priority one and a 30-second wait.
func (g *GrpcStarter) Setting() *parent.Setting {
	if g.GrpcSetting != nil {
		return g.GrpcSetting
	}
	return parent.NewSetting("gRPC-Starter", false, 1, false, time.Second*30, func(instance any) {
		config := g.getConfig()
		if config.InitFunc != nil {
			config.InitFunc(instance.(*grpc.Server))
		}
	})
}

func serverTraceInterceptor(traceIdSupplier TraceIdSupplier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if vals := md.Get(traceIdKey); len(vals) > 0 && traceIdSupplier != nil {
			traceIdSupplier.SetTraceId(vals[0])
		}
		return handler(ctx, req)
	}
}

// Start binds the listener and returns the raw server; serving begins asynchronously.
// Only one server can run per process. Startup does not register discovery endpoints.
func (g *GrpcStarter) Start() (any, error) {
	config := g.getConfig()
	grpcLifecycleLock.Lock()
	if grpcState != grpcStopped {
		runtime := grpcRuntimeState.Load()
		grpcLifecycleLock.Unlock()
		if runtime != nil {
			return runtime.server, ErrGrpcServerAlreadyStarted
		}
		return nil, ErrGrpcServerAlreadyStarted
	}
	grpcState = grpcStarting
	grpcLifecycleLock.Unlock()
	started := false
	defer func() {
		if !started {
			grpcLifecycleLock.Lock()
			grpcState = grpcStopped
			grpcLifecycleLock.Unlock()
		}
	}()

	var server *grpc.Server
	if config.TraceIdSupplier != nil {
		server = grpc.NewServer(grpc.UnaryInterceptor(serverTraceInterceptor(config.TraceIdSupplier)))
	} else {
		server = grpc.NewServer()
	}
	if config.RegisterService != nil {
		config.RegisterService(server)
	}

	lis, err := net.Listen(config.Network, config.ListenAddress)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	runtime := &grpcRuntime{server: server, done: done}
	grpcLifecycleLock.Lock()
	grpcRuntimeState.Store(runtime)
	grpcState = grpcRunning
	grpcLifecycleLock.Unlock()
	started = true

	go func() {
		defer close(done)
		_ = server.Serve(lis)
		clearGrpcServer(runtime)
	}()
	return server, nil
}

// Stop drains RPCs until maxWaitTime, then forces termination and returns ErrGrpcStopTimeout.
// Raw access becomes unavailable immediately; caller-owned client connections are unaffected.
func (g *GrpcStarter) Stop(maxWaitTime time.Duration) (gracefully, stopped bool, err error) {
	grpcLifecycleLock.Lock()
	runtime := grpcRuntimeState.Load()
	if grpcState != grpcRunning || runtime == nil {
		grpcLifecycleLock.Unlock()
		return false, true, ErrGrpcServerNotStarted
	}
	grpcRuntimeState.Store(nil)
	grpcState = grpcStopping
	grpcLifecycleLock.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.server.GracefulStop()
	}()
	timer := time.NewTimer(maxWaitTime)
	defer timer.Stop()
	select {
	case <-done:
		gracefully = true
		stopped = true
	case <-timer.C:
		runtime.server.Stop()
		gracefully = false
		stopped = true
		err = ErrGrpcStopTimeout
	}
	grpcLifecycleLock.Lock()
	grpcState = grpcStopped
	grpcLifecycleLock.Unlock()
	return
}

// RawGrpcServer returns the starter-owned server, or nil outside the running state.
// Register services through RegisterService; the starter owns server shutdown.
func RawGrpcServer() *grpc.Server {
	runtime := grpcRuntimeState.Load()
	if runtime == nil {
		return nil
	}
	return runtime.server
}

func clearGrpcServer(runtime *grpcRuntime) {
	if !grpcRuntimeState.CompareAndSwap(runtime, nil) {
		return
	}
	grpcLifecycleLock.Lock()
	if grpcState == grpcRunning {
		grpcState = grpcStopped
	}
	grpcLifecycleLock.Unlock()
}
