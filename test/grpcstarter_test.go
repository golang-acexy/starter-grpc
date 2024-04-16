package test

import (
	"fmt"
	"github.com/golang-acexy/starter-grpc/grpcmodule"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"github.com/golang-acexy/starter-parent/parentmodule/declaration"
	"google.golang.org/grpc"
	"testing"
	"time"
)

var moduleLoaders []declaration.ModuleLoader
var gModule *grpcmodule.GrpcModule

func init() {

	gModule = &grpcmodule.GrpcModule{}
	gModule.GrpcInterceptor = func(instance *grpc.Server) {
		// 使用加载参数拦截注入服务
		pbuser.RegisterUserServiceServer(instance, &pbuser.UserServiceImpl{})
	}

	//// 注册实际业务实现
	//gModule.RegisterService(func(server *grpc.Server) {
	//	pbuser.RegisterUserServiceServer(server, &pbuser.UserServiceImpl{})
	//})

	moduleLoaders = []declaration.ModuleLoader{gModule}
}

func TestLoadAndUnload(t *testing.T) {
	m := declaration.Module{
		ModuleLoaders: moduleLoaders,
	}
	err := m.Load()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	time.Sleep(time.Second * 2)
	m.Unload(10)
}

// 启动服务端
func TestStartSrv(t *testing.T) {
	m := declaration.Module{
		ModuleLoaders: moduleLoaders,
	}
	err := m.Load()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	select {}
}

// 启动一批服务端 8082 - 8085
func TestStartMoreSrv(t *testing.T) {

	grpcInterface := func(instance *grpc.Server) {
		pbuser.RegisterUserServiceServer(instance, &pbuser.UserServiceImpl{})
	}

	gModule1 := &grpcmodule.GrpcModule{
		GrpcInterceptor: grpcInterface,
		ListenAddress:   ":8082",
	}

	gModule2 := &grpcmodule.GrpcModule{
		GrpcInterceptor: grpcInterface,
		ListenAddress:   ":8083",
	}

	gModule3 := &grpcmodule.GrpcModule{
		GrpcInterceptor: grpcInterface,
		ListenAddress:   ":8084",
	}

	gModule4 := &grpcmodule.GrpcModule{
		GrpcInterceptor: grpcInterface,
		ListenAddress:   ":8085",
	}

	loaders := []declaration.ModuleLoader{gModule1, gModule2, gModule3, gModule4}
	m := declaration.Module{
		ModuleLoaders: loaders,
	}

	err := m.Load()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	select {}
}
