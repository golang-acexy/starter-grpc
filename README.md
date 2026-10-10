# starter-grpc

`starter-grpc` is the gRPC starter for the golang-acexy starter/cloud ecosystem. It wraps `google.golang.org/grpc` and provides server lifecycle management, client creation, trace-id propagation, and resolver-based service discovery.

## Ecosystem Role

This module provides the RPC transport layer. It can operate with direct targets or resolver implementations, and it composes with `starter-nacos` when services use Nacos registration and discovery.

## Requirements

Current module Go version: `1.26.7`.

## Installation

```bash
go get github.com/golang-acexy/starter-grpc
```

## Server Design

The server side is intentionally singleton-based. One application process owns one `grpc.Server`; multiple business services should be registered into that server through `GrpcConfig.RegisterService`. Starting another `GrpcStarter` in the same process returns `ErrGrpcServerAlreadyStarted`.

This matches the normal service model: one process exposes one gRPC endpoint, while multiple instances should be represented by multiple processes, not multiple listeners inside one process.

## Server Usage

```go
starter := &grpcstarter.GrpcStarter{
    Config: grpcstarter.GrpcConfig{
        Network:       "tcp",
        ListenAddress: ":8081",
        RegisterService: func(server *grpc.Server) {
            // pb.RegisterUserServiceServer(server, userService)
            // pb.RegisterOrderServiceServer(server, orderService)
        },
    },
}

loader := parent.InitStarterLoader([]parent.Starter{starter})
if err := loader.Start(); err != nil {
    panic(err)
}
```

Use `InitFunc` when initialization needs the raw `*grpc.Server` after startup. Use `TraceIdSupplier` to bind trace-id propagation through unary interceptors. Its implementation must isolate concurrent RPC trace state and manage cleanup. Tracing is chained with caller client interceptors and does not cover streaming RPCs.

## Client Usage

Direct connection:

```go
client, err := grpcstarter.NewClientConn(
    "localhost:8081",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
    return err
}
defer client.CloseConn()
```

Static resolver:

```go
r := resolver.NewStaticResolver([]string{
    "127.0.0.1:8081",
    "127.0.0.1:8082",
})

client, err := grpcstarter.NewClientConnWithResolver(
    resolver.StaticScheme+":///users",
    r,
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
)
if err != nil {
    return err
}
defer client.CloseConn()
```

Etcd and Nacos resolvers are available through `resolver.NewEtcdResolver(...)` and `resolver.NewNacosResolver(...)` for dynamic discovery.

Static resolver updates are concurrency-safe and are delivered to every active client connection created from the resolver. Each Nacos-backed resolver owns an independent subscription lifecycle, so closing one gRPC connection does not cancel another connection's watch. Nacos updates retain only healthy, enabled instances with positive weight, and received empty updates withdraw prior addresses. Enable Nacos SDK `UpdateCacheWhenEmpty` to receive last-instance removal notifications. IPv6 addresses are supported.

Etcd resolvers use the endpoint watch's initial snapshot and subsequent updates. Nacos and Etcd SDK clients remain caller-owned: close gRPC connections first, then close shared discovery clients. This changes the previous Etcd behavior, which closed the supplied client when a resolver closed.

## Common API

- `GrpcStarter.Start()` starts the singleton gRPC server.
- `GrpcStarter.Stop(maxWaitTime)` gracefully stops the server, then forces stop on timeout.
- `RawGrpcServer()` returns the singleton raw server.
- `NewClientConn(...)` creates a direct client connection.
- `NewClientConnWithTraceSupplier(...)` creates a direct client with trace propagation.
- `NewClientConnWithResolver(...)` creates a client using service discovery.

## Verification

Package-level regression tests use in-memory RPC transport and fake discovery services; no Nacos or Etcd server is required:

```bash
go test -race ./grpcstarter/...
```

Direct client/server communication and static discovery also have local RPC tests under `test/client`.

Etcd and Nacos resolver tests require external services and are skipped by default:

```bash
STARTER_GRPC_ETCD_TEST=1 \
STARTER_GRPC_ETCD_ENDPOINT=http://localhost:2379 \
go test ./test/client -run TestCallServerWithEtcdResolver

STARTER_GRPC_NACOS_TEST=1 \
STARTER_GRPC_NACOS_HOST=localhost \
STARTER_GRPC_NACOS_PORT=8848 \
STARTER_GRPC_NACOS_SERVICE=go \
go test ./test/client -run TestCallServerWithNacosResolver
```

## Lifecycle and Design Notes

Do not create multiple `GrpcStarter` instances to listen on different ports in the same process. Register multiple services into one server instead.

The standard gRPC starter does not allow parent-managed restart after successful shutdown. Client connections are caller-owned and should be closed independently of the server starter. Connection construction is lazy and does not guarantee remote readiness; apply deadlines to RPC contexts. `InitFunc` is called by the default parent setting, so overriding `GrpcSetting` replaces that callback. Configuration is resolved once with `LazyConfig` precedence; defaults are `tcp` and `:8081`.
