package server

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"google.golang.org/grpc"
)

func stopExistingGrpcServer() {
	if grpcstarter.RawGrpcServer() != nil {
		_, _, _ = (&grpcstarter.GrpcStarter{}).Stop(time.Second)
	}
}

func TestGrpcStarterLifecycle(t *testing.T) {
	stopExistingGrpcServer()
	defer stopExistingGrpcServer()

	registered := false
	starter := &grpcstarter.GrpcStarter{
		Config: grpcstarter.GrpcConfig{
			ListenAddress: "127.0.0.1:0",
			RegisterService: func(server *grpc.Server) {
				registered = true
				pbuser.RegisterUserServiceServer(server, &pbuser.UserServiceImpl{})
			},
		},
	}

	instance, err := starter.Start()
	if err != nil {
		t.Fatalf("start grpc starter failed: %v", err)
	}
	if instance == nil || grpcstarter.RawGrpcServer() == nil {
		t.Fatal("grpc server should be initialized")
	}
	if !registered {
		t.Fatal("register service should be called")
	}

	gracefully, stopped, err := starter.Stop(time.Second)
	if err != nil {
		t.Fatalf("stop grpc starter failed: %v", err)
	}
	if !gracefully || !stopped {
		t.Fatalf("unexpected stop result, gracefully=%v stopped=%v", gracefully, stopped)
	}
	if grpcstarter.RawGrpcServer() != nil {
		t.Fatal("grpc server should be cleared after stop")
	}
}

func TestGrpcStarterRejectsDuplicateServer(t *testing.T) {
	stopExistingGrpcServer()
	defer stopExistingGrpcServer()

	first := &grpcstarter.GrpcStarter{Config: grpcstarter.GrpcConfig{ListenAddress: "127.0.0.1:0"}}
	if _, err := first.Start(); err != nil {
		t.Fatalf("start first grpc server failed: %v", err)
	}
	defer first.Stop(time.Second)

	second := &grpcstarter.GrpcStarter{Config: grpcstarter.GrpcConfig{ListenAddress: "127.0.0.1:0"}}
	_, err := second.Start()
	if !errors.Is(err, grpcstarter.ErrGrpcServerAlreadyStarted) {
		t.Fatalf("expected ErrGrpcServerAlreadyStarted, got %v", err)
	}
}

func TestGrpcStarterStopBeforeStart(t *testing.T) {
	stopExistingGrpcServer()

	starter := &grpcstarter.GrpcStarter{}
	_, _, err := starter.Stop(time.Second)
	if !errors.Is(err, grpcstarter.ErrGrpcServerNotStarted) {
		t.Fatalf("expected ErrGrpcServerNotStarted, got %v", err)
	}
}
