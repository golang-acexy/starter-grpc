package test

import (
	"context"
	"fmt"
	"github.com/acexy/golang-toolkit/sys"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

// 使用静态服务端列表 启动 grpcstarter_test.go -> TestStartMoreSrv 启动一批服务端
func TestCallServerWithStaticResolver(t *testing.T) {
	r := resolver.NewStaticResolver([]string{
		"127.0.0.1:8084",
		"127.0.0.1:8083",
		"127.0.0.1:8085",
		"127.0.0.1:8082",
		"127.0.0.1:8081",
	})
	conn, err := grpcstarter.NewClientConnWithResolver(resolver.StaticScheme+":///users", r,
		grpc.WithTransportCredentials(insecure.NewCredentials()),               // 免认证
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // 使用负载策略 (如果不使用负载策略则不会在服务器列表中使用负载功能，可能一直请求同一个服务器)
	)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	go func() {
		time.Sleep(time.Second * 5)
		fmt.Println("移除其他实例，只保留8084")
		r.Update([]string{"127.0.0.1:8084"})
		time.Sleep(time.Second * 5)
		fmt.Println("增加实例，保留 8081 8082")
		r.Update([]string{"127.0.0.1:8081", "127.0.0.1:8082"})
	}()
	doRequest(context.Background(), conn)
	sys.ShutdownHolding()
}
