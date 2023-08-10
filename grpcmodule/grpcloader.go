package grpcmodule

import (
	"github.com/acexy/golang-toolkit/log"
	"github.com/golang-acexy/starter-parent/parentmodule/declaration"
	"google.golang.org/grpc"
	"net"
	"time"
)

var server *grpc.Server

type GrpcModule struct {

	// grpc listener
	Network string
	Address string

	GrpcModuleConfig *declaration.ModuleConfig
	GrpcInterface    *func(instance interface{})
}

func (g *GrpcModule) ModuleConfig() *declaration.ModuleConfig {
	if g.GrpcModuleConfig != nil {
		return g.GrpcModuleConfig
	}
	return &declaration.ModuleConfig{
		ModuleName:               "gRPC",
		UnregisterPriority:       0,
		UnregisterAllowAsync:     true,
		UnregisterMaxWaitSeconds: 30,
	}
}

func (g *GrpcModule) Register(interceptor *func(instance interface{})) error {
	server = grpc.NewServer()
	if interceptor != nil {
		(*g.GrpcInterface)(server)
	}

	lis, err := net.Listen(g.Network, g.Address)
	if err != nil {
		return err
	}

	if g.Network == "" {
		g.Network = "tcp"
	}

	if g.Address == "" {
		g.Address = ":8081"
	}

	go func() {
		log.Logrus().Traceln("gRPC will listen at ", g.Address)
		if err = server.Serve(lis); err != nil {
			log.Logrus().WithError(err).Traceln("gRPC stopped")
		}
	}()

	return nil
}

// Interceptor 初始化gin原始实例拦截器
// request instance: *grpc.Server
func (g *GrpcModule) Interceptor() *func(instance interface{}) {
	return g.GrpcInterface
}

func (g *GrpcModule) Unregister(maxWaitSeconds uint) (gracefully bool, err error) {

	done := make(chan bool)
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
