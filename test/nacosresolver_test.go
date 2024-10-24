package test

import (
	"context"
	"fmt"
	"github.com/acexy/golang-toolkit/sys"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
)

func TestCallServerWithNacosResolver(t *testing.T) {
	client, _ := clients.NewNamingClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{
			{
				IpAddr: "localhost",
				Port:   8848,
			},
		},
		ClientConfig: &constant.ClientConfig{
			NamespaceId:         "demo",
			Username:            "nacos",
			Password:            "nacos",
			LogLevel:            "error",
			LogDir:              "./",
			CacheDir:            "./",
			NotLoadCacheAtStart: true,
		},
	})

	nacosResolver := resolver.NewNacosResolver(client, "CLOUD")
	conn, err := grpcstarter.NewClientConnWithResolver(resolver.NacosScheme+":///go", nacosResolver,
		grpc.WithTransportCredentials(insecure.NewCredentials()),               // 免认证
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // 使用负载策略 (如果不使用负载策略则不会在服务器列表中使用负载功能，可能一直请求同一个服务器)
	)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	doRequest(context.Background(), conn)
	sys.ShutdownHolding()
}
