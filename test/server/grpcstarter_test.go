package server

import (
	"fmt"
	"github.com/acexy/golang-toolkit/sys"
	"github.com/acexy/golang-toolkit/util/json"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"github.com/golang-acexy/starter-parent/parent"
	"google.golang.org/grpc"
	"testing"
	"time"
)

var starterLoader *parent.StarterLoader
var grpcStarter *grpcstarter.GrpcStarter

func init() {
	grpcStarter = &grpcstarter.GrpcStarter{}

	// 使用初始化函数
	//grpcStarter.Config.InitFunc = func(instance *grpc.Server) {
	//}

	grpcStarter.Config.RegisterService = func(g *grpc.Server) {
		pbuser.RegisterUserServiceServer(g, &pbuser.UserServiceImpl{})
	}

	starterLoader = parent.NewStarterLoader([]parent.Starter{grpcStarter})
}

func TestLoadAndUnload(t *testing.T) {
	err := starterLoader.Start()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	time.Sleep(time.Second * 2)
	stopResult, err := starterLoader.Stop(time.Second * 10)
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	fmt.Println(json.ToJsonFormat(stopResult))
}

// 启动服务端
func TestStartSrv(t *testing.T) {
	err := starterLoader.Start()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	sys.ShutdownHolding()
}

// 启动一批服务端 8082 - 8085
func TestStartMoreSrv(t *testing.T) {

	registerService := func(instance *grpc.Server) {
		pbuser.RegisterUserServiceServer(instance, &pbuser.UserServiceImpl{})
	}

	gModule1 := &grpcstarter.GrpcStarter{
		Config: grpcstarter.GrpcConfig{
			RegisterService: registerService,
			ListenAddress:   ":8082",
		},
	}

	gModule2 := &grpcstarter.GrpcStarter{
		Config: grpcstarter.GrpcConfig{
			RegisterService: registerService,
			ListenAddress:   ":8083",
		},
	}

	gModule3 := &grpcstarter.GrpcStarter{
		Config: grpcstarter.GrpcConfig{
			RegisterService: registerService,
			ListenAddress:   ":8084",
		},
	}

	gModule4 := &grpcstarter.GrpcStarter{
		Config: grpcstarter.GrpcConfig{
			RegisterService: registerService,
			ListenAddress:   ":8085",
		},
	}
	starterLoader.AddStarter(gModule1, gModule2, gModule3, gModule4)

	err := starterLoader.Start()
	if err != nil {
		fmt.Printf("%+v\n", err)
		return
	}
	sys.ShutdownHolding()
}
