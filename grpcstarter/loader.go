package grpcstarter

import (
	"github.com/golang-acexy/starter-parent/parent"
	"google.golang.org/grpc"
	"net"
	"time"
)

var grpcServer *grpc.Server

type GrpcStarter struct {

	// grpc listener
	Network       string
	ListenAddress string

	InitFunc func(instance *grpc.Server)

	// 注册服务
	RegisterService func(g *grpc.Server)

	GrpcSetting *parent.Setting
}

func (g *GrpcStarter) Setting() *parent.Setting {
	if g.GrpcSetting != nil {
		return g.GrpcSetting
	}
	return parent.NewSetting("gRPC-Starter", 0, false, time.Second*30, func(instance interface{}) {
		if g.InitFunc != nil {
			g.InitFunc(instance.(*grpc.Server))
		}
	})
}

func (g *GrpcStarter) Start() (interface{}, error) {
	grpcServer = grpc.NewServer()

	if g.Network == "" {
		g.Network = "tcp"
	}

	if g.ListenAddress == "" {
		g.ListenAddress = ":8081"
	}

	// 注册用户服务实现
	if g.RegisterService != nil {
		g.RegisterService(grpcServer)
	}

	lis, err := net.Listen(g.Network, g.ListenAddress)
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
