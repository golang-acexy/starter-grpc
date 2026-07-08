package client

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	etcdClient "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestCallServerWithEtcdResolver verifies etcd resolver integration.
//
// It requires a local or configured etcd instance and is disabled by default:
// STARTER_GRPC_ETCD_TEST=1 STARTER_GRPC_ETCD_ENDPOINT=http://localhost:2379 go test ./test/client -run TestCallServerWithEtcdResolver
func TestCallServerWithEtcdResolver(t *testing.T) {
	if os.Getenv("STARTER_GRPC_ETCD_TEST") != "1" {
		t.Skip("set STARTER_GRPC_ETCD_TEST=1 to run etcd resolver integration test")
	}

	endpoint := os.Getenv("STARTER_GRPC_ETCD_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:2379"
	}

	address := startUserGrpcServer(t)
	client, err := etcdClient.NewFromURL(endpoint)
	if err != nil {
		t.Fatalf("create etcd client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	registerInstanceToEtcd(t, client, ctx, "users", "test-instance", address, 30)

	etcdResolver := resolver.NewEtcdResolver(client)
	conn, err := grpcstarter.NewClientConnWithResolver(
		resolver.EtcdScheme+":///users",
		etcdResolver,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		t.Fatalf("create etcd resolver grpc client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.CloseConn()
	})

	assertUserServiceCall(t, conn)
}

func registerInstanceToEtcd(t *testing.T, client *etcdClient.Client, ctx context.Context, target, instanceId, address string, ttl int64) {
	t.Helper()
	manager, err := endpoints.NewManager(client, target)
	if err != nil {
		t.Fatalf("create etcd endpoint manager failed: %v", err)
	}
	key := target
	if strings.HasSuffix(key, "/") {
		key += instanceId
	} else {
		key += "/" + instanceId
	}
	lease, err := client.Grant(context.Background(), ttl)
	if err != nil {
		t.Fatalf("grant etcd lease failed: %v", err)
	}
	if err := manager.AddEndpoint(context.Background(), key, endpoints.Endpoint{Addr: address}, etcdClient.WithLease(lease.ID)); err != nil {
		t.Fatalf("add etcd endpoint failed: %v", err)
	}
	alive, err := client.KeepAlive(ctx, lease.ID)
	if err != nil {
		t.Fatalf("keep alive etcd endpoint failed: %v", err)
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-alive:
				if !ok {
					return
				}
			}
		}
	}()
	time.Sleep(200 * time.Millisecond)
}
