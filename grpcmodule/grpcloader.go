package grpcmodule

import (
	"github.com/acexy/golang-toolkit/logger"
	"github.com/golang-acexy/starter-parent/parentmodule/declaration"
	"google.golang.org/grpc"
	"net"
	"time"
)

const (
	defaultNetwork       = "tcp"
	defaultListenAddress = ":8081"
)

var server *grpc.Server

type GrpcModule struct {

	// grpc listener
	Network       string
	ListenAddress string

	GrpcModuleConfig *declaration.ModuleConfig

	GrpcInterceptor func(instance *grpc.Server)

	registerService func(*grpc.Server)
}

func (g *GrpcModule) ModuleConfig() *declaration.ModuleConfig {
	if g.GrpcModuleConfig != nil {
		return g.GrpcModuleConfig
	}
	return &declaration.ModuleConfig{
		ModuleName:               "GRpc",
		UnregisterPriority:       0,
		UnregisterAllowAsync:     true,
		UnregisterMaxWaitSeconds: 30,
		LoadInterceptor: func(instance interface{}) {
			if g.GrpcInterceptor != nil {
				g.GrpcInterceptor(instance.(*grpc.Server))
			}
		},
	}
}

func (g *GrpcModule) Register() (interface{}, error) {
	server = grpc.NewServer()

	if g.Network == "" {
		g.Network = defaultNetwork
	}

	if g.ListenAddress == "" {
		g.ListenAddress = defaultListenAddress
	}

	// 注册用户服务实现
	if g.registerService != nil {
		g.registerService(server)
	}

	lis, err := net.Listen(g.Network, g.ListenAddress)
	if err != nil {
		return nil, err
	}

	go func() {
		logger.Logrus().Traceln(g.ModuleConfig().ModuleName, "started")
		if err = server.Serve(lis); err != nil {
		}
	}()

	return server, nil
}

func (g *GrpcModule) RegisterService(fn func(*grpc.Server)) {
	g.registerService = fn
}

func (g *GrpcModule) Unregister(maxWaitSeconds uint) (gracefully bool, err error) {
	done := make(chan interface{})
	go func() {
		server.GracefulStop()
		done <- true
	}()
	select {
	case <-done:
		gracefully = true
	case <-time.After(time.Second * time.Duration(maxWaitSeconds)):
		gracefully = false
	}
	return
}
