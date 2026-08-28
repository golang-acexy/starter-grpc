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

type TraceIdSupplier interface {
	SetTraceId(traceId string)
	GetTraceId() string
}

type GrpcConfig struct {
	// grpc listener
	Network       string
	ListenAddress string
	InitFunc      func(instance *grpc.Server)
	// 链路追踪TraceId日志实现
	TraceIdSupplier TraceIdSupplier
	// 注册服务
	RegisterService func(g *grpc.Server)
}

type GrpcStarter struct {
	Config      GrpcConfig
	LazyConfig  func() GrpcConfig
	config      *GrpcConfig
	configOnce  sync.Once
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

// RawGrpcServer 获取原始grpc server实例
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
