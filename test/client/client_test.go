package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/acexy/golang-toolkit/logger"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/test"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func nextLocalAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate local address failed: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close local address listener failed: %v", err)
	}
	return address
}

func startUserGrpcServer(t *testing.T) string {
	t.Helper()
	stopExistingGrpcServer(t)

	address := nextLocalAddress(t)
	starter := &grpcstarter.GrpcStarter{
		Config: grpcstarter.GrpcConfig{
			ListenAddress:   address,
			TraceIdSupplier: test.GetTraceIdSupplier(),
			RegisterService: func(server *grpc.Server) {
				pbuser.RegisterUserServiceServer(server, &pbuser.UserServiceImpl{})
			},
		},
	}
	if _, err := starter.Start(); err != nil {
		t.Fatalf("start grpc server failed: %v", err)
	}
	t.Cleanup(func() {
		_, _, _ = starter.Stop(time.Second)
	})
	return address
}

func stopExistingGrpcServer(t *testing.T) {
	t.Helper()
	if grpcstarter.RawGrpcServer() != nil {
		if _, _, err := (&grpcstarter.GrpcStarter{}).Stop(time.Second); err != nil {
			t.Fatalf("stop existing grpc server failed: %v", err)
		}
	}
}

func assertUserServiceCall(t *testing.T, conn *grpcstarter.GrpcClient) {
	t.Helper()
	client := pbuser.NewUserServiceClient(conn.GetRawConn())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	logger.Logrus().Debugln("query user service")
	response, err := client.QueryById(ctx, &pbuser.Request{Id: 1})
	if err != nil {
		t.Fatalf("query user service failed: %v", err)
	}
	if len(response.GetUsers()) == 0 {
		t.Fatal("query user service should return users")
	}
}

// TestCallServer verifies direct client/server communication.
func TestCallServer(t *testing.T) {
	address := startUserGrpcServer(t)
	logger.SetTraceIdSupplier(test.GetTraceIdSupplier()) // 实现traceId传递
	conn, err := grpcstarter.NewClientConnWithTraceSupplier(
		address,
		test.GetTraceIdSupplier(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create direct grpc client failed: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.CloseConn()
	})
	assertUserServiceCall(t, conn)
}
