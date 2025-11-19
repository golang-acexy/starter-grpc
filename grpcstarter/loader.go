package grpcstarter

import (
	"context"
	"net"
	"time"

	"github.com/golang-acexy/starter-parent/parent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const traceIdKey = "trace-id"

var grpcServer *grpc.Server
var supplier TraceIdSupplier

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
		// 注册用户服务实现
		if config.RegisterService != nil {
			if config.TraceIdSupplier != nil {
				supplier = config.TraceIdSupplier
				grpcServer = grpc.NewServer(grpc.UnaryInterceptor(serverTraceInterceptor))
			} else {
				grpcServer = grpc.NewServer()
			}
			config.RegisterService(grpcServer)
		}
		g.config = &config
	}
	return g.config
}

func (g *GrpcStarter) Setting() *parent.Setting {
	if g.GrpcSetting != nil {
		return g.GrpcSetting
	}
	return parent.NewSetting("gRPC-Starter", 1, false, time.Second*30, func(instance interface{}) {
		config := g.getConfig()
		if config.InitFunc != nil {
			config.InitFunc(instance.(*grpc.Server))
		}
	})
}

func serverTraceInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if vals := md.Get(traceIdKey); len(vals) > 0 {
		supplier.SetTraceId(vals[0])
	}
	return handler(ctx, req)
}

func (g *GrpcStarter) Start() (interface{}, error) {
	config := g.getConfig()
	lis, err := net.Listen(config.Network, config.ListenAddress)
	if err != nil {
		return nil, err
	}
	errChn := make(chan error)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			errChn <- err
		}
	}()
	select {
	case <-time.After(time.Second):
		return grpcServer, nil
	case err = <-errChn:
		return grpcServer, err
	}
}

func (g *GrpcStarter) Stop(maxWaitTime time.Duration) (gracefully, stopped bool, err error) {
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		done <- struct{}{}
	}()
	select {
	case <-done:
		gracefully = true
		stopped = true
	case <-time.After(maxWaitTime):
		gracefully = false
		stopped = true
	}
	return
}

// RawGrpcServer 获取原始grpc server实例
func RawGrpcServer() *grpc.Server {
	return grpcServer
}
