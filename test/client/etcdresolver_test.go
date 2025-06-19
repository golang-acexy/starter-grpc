package client

import (
	"context"
	"fmt"
	"github.com/acexy/golang-toolkit/logger"
	"github.com/acexy/golang-toolkit/sys"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	etcdClient "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"strings"
	"testing"
	"time"
)

// 使用动态服务器路由列表(基于etcd) 启动 grpcstarter_test.go -> TestStartMoreSrv 启动一批服务端
func TestCallServerWithEtcdResolver(t *testing.T) {
	client, _ := etcdClient.NewFromURLs([]string{"http://localhost:2379"})
	etcdResolver := resolver.NewEtcdResolver(client)
	conn, err := grpcstarter.NewClientConnWithResolver(resolver.EtcdScheme+":///users", etcdResolver,
		grpc.WithTransportCredentials(insecure.NewCredentials()),               // 免认证
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // 使用负载策略 (如果不使用负载策略则不会在服务器列表中使用负载功能，可能一直请求同一个服务器)
	)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	// 开启一个异步协程，5秒后，将相关服务端实例注册到etcd，测试本客户端是否可以感知并开始请求
	go func() {

		// 也可以通过直接操作etcd将相关服务端进行注册
		// etcdctl get "" --prefix=true 查看所有key
		// 手动注册服务端实例至etcd
		// etcdctl put "users/1" '{"Addr":"localhost:8085"}'
		// etcdctl put "users/2" '{"Addr":"localhost:8084"}'
		// etcdctl put "users/3" '{"Addr":"localhost:8083"}'

		ctx, cancel := context.WithCancel(context.Background())
		time.Sleep(time.Second * 5)
		fmt.Println("register new instance")
		registerInstanceToEtcd(client, ctx, "users", "1", "localhost:8085", 3)
		time.Sleep(time.Second * 5)
		//registerInstanceToEtcd(client, ctx, "users", "2", "localhost:8084", 3)
		//time.Sleep(time.Second * 5)
		//registerInstanceToEtcd(client, ctx, "users", "3", "localhost:8083", 3)
		//time.Sleep(time.Second * 5)
		//registerInstanceToEtcd(client, ctx, "users", "4", "localhost:8082", 10)
		time.Sleep(time.Second * 10)
		fmt.Println("stop instance keepalive")
		cancel()
	}()

	doRequest(context.Background(), conn)
	sys.ShutdownHolding()
}

// registerInstanceToEtcd 向etcd服务器中注册服务实例 以便于其他客户端可以动态感知
// ttl (s) 租约续期时间 如果在指定时间未续约，etcd将取消注册，其他客户端将无法感知该实例
func registerInstanceToEtcd(client *etcdClient.Client, ctx context.Context, target, instanceId, address string, ttl int64) {
	manager, err := endpoints.NewManager(client, target)
	if strings.HasSuffix(target, "/") {
		target += instanceId
	} else {
		target += "/" + instanceId
	}
	lease, err := client.Grant(context.TODO(), ttl)
	if err != nil {
		logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
		return
	}
	err = manager.AddEndpoint(context.TODO(), target, endpoints.Endpoint{Addr: address}, etcdClient.WithLease(lease.ID))
	if err != nil {
		logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
		return
	}
	alive, err := client.KeepAlive(ctx, lease.ID)
	if err != nil {
		logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				break
			case <-alive:

			}
		}
	}()
}
