package grpcstarter

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/golang-acexy/starter-parent/parent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const traceIdKey = "trace-id"

var grpcServer *grpc.Server
var grpcServerLock sync.RWMutex

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
	GrpcSetting *parent.Setting
}

func (g *GrpcStarter) getConfig() *GrpcConfig {
	if g.config == nil {
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
	}
	return g.config
}

func (g *GrpcStarter) Setting() *parent.Setting {
	if g.GrpcSetting != nil {
		return g.GrpcSetting
	}
	return parent.NewSetting("gRPC-Starter", 1, false, time.Second*30, func(instance any) {
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
	grpcServerLock.Lock()
	if grpcServer != nil {
		server := grpcServer
		grpcServerLock.Unlock()
		return server, ErrGrpcServerAlreadyStarted
	}
	var server *grpc.Server
	if config.TraceIdSupplier != nil {
		server = grpc.NewServer(grpc.UnaryInterceptor(serverTraceInterceptor(config.TraceIdSupplier)))
	} else {
		server = grpc.NewServer()
	}
	if config.RegisterService != nil {
		config.RegisterService(server)
	}
	grpcServer = server
	grpcServerLock.Unlock()

	lis, err := net.Listen(config.Network, config.ListenAddress)
	if err != nil {
		clearGrpcServer(server)
		return nil, err
	}
	errChn := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(lis); serveErr != nil {
			errChn <- serveErr
		}
	}()
	select {
	case <-time.After(time.Second):
		return server, nil
	case err = <-errChn:
		clearGrpcServer(server)
		return server, err
	}
}

func (g *GrpcStarter) Stop(maxWaitTime time.Duration) (gracefully, stopped bool, err error) {
	server := RawGrpcServer()
	if server == nil {
		return false, true, ErrGrpcServerNotStarted
	}
	done := make(chan struct{}, 1)
	go func() {
		server.GracefulStop()
		done <- struct{}{}
	}()
	select {
	case <-done:
		gracefully = true
		stopped = true
	case <-time.After(maxWaitTime):
		server.Stop()
		gracefully = false
		stopped = true
		err = ErrGrpcStopTimeout
	}
	clearGrpcServer(server)
	return
}

// RawGrpcServer 获取原始grpc server实例
func RawGrpcServer() *grpc.Server {
	grpcServerLock.RLock()
	defer grpcServerLock.RUnlock()
	return grpcServer
}

func clearGrpcServer(server *grpc.Server) {
	grpcServerLock.Lock()
	defer grpcServerLock.Unlock()
	if grpcServer == server {
		grpcServer = nil
	}
}
