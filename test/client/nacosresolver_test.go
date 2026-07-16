package client

import (
	"os"
	"strconv"
	"testing"

	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestCallServerWithNacosResolver verifies Nacos resolver integration.
//
// It requires an existing Nacos service instance and is disabled by default:
// STARTER_GRPC_NACOS_TEST=1 STARTER_GRPC_NACOS_SERVICE=go go test ./test/client -run TestCallServerWithNacosResolver
func TestCallServerWithNacosResolver(t *testing.T) {
	if os.Getenv("STARTER_GRPC_NACOS_TEST") != "1" {
		t.Skip("set STARTER_GRPC_NACOS_TEST=1 to run nacos resolver integration test")
	}

	port := uint64(8848)
	if rawPort := os.Getenv("STARTER_GRPC_NACOS_PORT"); rawPort != "" {
		parsed, err := strconv.ParseUint(rawPort, 10, 64)
		if err != nil {
			t.Fatalf("parse STARTER_GRPC_NACOS_PORT failed: %v", err)
		}
		port = parsed
	}
	host := os.Getenv("STARTER_GRPC_NACOS_HOST")
	if host == "" {
		host = "localhost"
	}
	group := os.Getenv("STARTER_GRPC_NACOS_GROUP")
	if group == "" {
		group = "DEFAULT_GROUP"
	}
	service := os.Getenv("STARTER_GRPC_NACOS_SERVICE")
	if service == "" {
		service = "go"
	}

	client, err := clients.NewNamingClient(vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{{IpAddr: host, Port: port}},
		ClientConfig: &constant.ClientConfig{
			Username:            os.Getenv("STARTER_GRPC_NACOS_USERNAME"),
			Password:            os.Getenv("STARTER_GRPC_NACOS_PASSWORD"),
			LogLevel:            "warn",
			LogDir:              "./",
			CacheDir:            "./",
			NotLoadCacheAtStart: true,
		},
	})
	if err != nil {
		t.Fatalf("create nacos client failed: %v", err)
	}

	nacosResolver := resolver.NewNacosResolver(client, group)
	conn, err := grpcstarter.NewClientConnWithResolver(
		resolver.NacosScheme+":///"+service,
		nacosResolver,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		t.Fatalf("create nacos resolver grpc client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.CloseConn()
	})

	assertUserServiceCall(t, conn)
}
