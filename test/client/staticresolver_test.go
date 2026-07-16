package client

import (
	"testing"

	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/grpcstarter/resolver"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/credentials/insecure"
)

// TestCallServerWithStaticResolver verifies client/server communication through the static resolver.
func TestCallServerWithStaticResolver(t *testing.T) {
	address := startUserGrpcServer(t)
	staticResolver := resolver.NewStaticResolver([]string{address})

	conn, err := grpcstarter.NewClientConnWithResolver(
		resolver.StaticScheme+":///users",
		staticResolver,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		t.Fatalf("create static resolver grpc client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.CloseConn()
	})

	assertUserServiceCall(t, conn)
}
