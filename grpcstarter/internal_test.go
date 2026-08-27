package grpcstarter

import (
	"testing"

	"google.golang.org/grpc"
)

func TestGrpcRuntimeSnapshotPublication(t *testing.T) {
	previous := grpcRuntimeState.Swap(nil)
	defer grpcRuntimeState.Store(previous)

	server := grpc.NewServer()
	runtime := &grpcRuntime{server: server, done: make(chan struct{})}
	grpcRuntimeState.Store(runtime)
	if RawGrpcServer() != server {
		t.Fatal("gRPC server 未从运行时快照读取")
	}

	grpcRuntimeState.Store(nil)
	if RawGrpcServer() != nil {
		t.Fatal("摘除运行时快照后不应继续暴露 gRPC server")
	}
}

func TestGrpcConfigResolvedOnce(t *testing.T) {
	resolveCount := 0
	starter := &GrpcStarter{LazyConfig: func() GrpcConfig {
		resolveCount++
		return GrpcConfig{}
	}}

	first := starter.getConfig()
	second := starter.getConfig()
	if first != second || resolveCount != 1 {
		t.Fatalf("配置应只解析一次: count=%d", resolveCount)
	}
	if first.Network != "tcp" || first.ListenAddress != ":8081" {
		t.Fatalf("默认配置异常: %+v", first)
	}
}
