package test

import (
	"context"
	"fmt"
	"github.com/acexy/golang-toolkit/math/random"
	"github.com/acexy/golang-toolkit/util"
	"github.com/golang-acexy/starter-grpc/grpcmodule"
	"github.com/golang-acexy/starter-grpc/grpcmodule/resolver"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

var userService pbuser.UserServiceClient

func doRequest(ctx context.Context, conn *grpcmodule.Conn) {
	if userService == nil {
		userService = pbuser.NewUserServiceClient(conn.GetConn())
	}
	go func() {
		for {
			userCall(userService)
			time.Sleep(time.Millisecond * 200)
			select {
			case <-ctx.Done():
				_ = conn.CloseConn()
				break
			default:
			}
		}
	}()
}

func userCall(userService pbuser.UserServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	user, err := userService.QueryById(ctx, &pbuser.Request{Id: uint64(random.RandInt(10))})
	if err != nil {
		statusError := status.Convert(err)
		fmt.Printf("%+v\n", statusError.Code())
		fmt.Printf("QueryById Error %T %+v\n", err, err)
		return
	}
	fmt.Println(util.ToJson(user))
}

// 使用直连的形式请求服务端
func TestCallServer(t *testing.T) {
	conn, err := grpcmodule.NewClientCoon("localhost:8081", true, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	doRequest(ctx, conn)
	<-ctx.Done()
}

// 使用静态服务端列表 启动 grpcstarter_test.go -> TestStartMoreSrv 启动一批服务端
func TestCallServerWithStaticResolver(t *testing.T) {
	conn, err := grpcmodule.NewClientConnWithResolver(resolver.StaticScheme+":///users", resolver.Static{Addresses: map[string][]string{
		"users": {
			"127.0.0.1:8085",
			"127.0.0.1:8084",
			"127.0.0.1:8083",
			"127.0.0.1:8082",
			"127.0.0.1:8081",
		},
	}}, true,
		grpc.WithTransportCredentials(insecure.NewCredentials()),               // 免认证
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // 使用负载策略 (如果不使用负载策略则不会在服务器列表中使用负载功能，可能一直请求同一个服务器)
	)
	if err != nil {
		fmt.Printf("%v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	doRequest(ctx, conn)
	<-ctx.Done()
}
