# starter-grpc

`starter-grpc` is the gRPC starter for the golang-acexy starter/cloud ecosystem. It wraps `google.golang.org/grpc` and provides server lifecycle management, client creation, trace-id propagation, and resolver-based service discovery.

## Requirements

Current module Go version: `1.25.8`.

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

Use `InitFunc` when initialization needs the raw `*grpc.Server` after startup. Use `TraceIdSupplier` to bind trace-id propagation through unary interceptors.

## Client Usage

Direct connection:

```go
client, err := grpcstarter.NewClientConn(
    "localhost:8081",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
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
```

Etcd and Nacos resolvers are available through `resolver.NewEtcdResolver(...)` and `resolver.NewNacosResolver(...)` for dynamic discovery.

## Common API

- `GrpcStarter.Start()` starts the singleton gRPC server.
- `GrpcStarter.Stop(maxWaitTime)` gracefully stops the server, then forces stop on timeout.
- `RawGrpcServer()` returns the singleton raw server.
- `NewClientConn(...)` creates a direct client connection.
- `NewClientConnWithTraceSupplier(...)` creates a direct client with trace propagation.
- `NewClientConnWithResolver(...)` creates a client using service discovery.

## Verification

Direct client/server communication and static resolver communication are covered by default tests:

```bash
go test ./test/client
```

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

## Notes

Do not create multiple `GrpcStarter` instances to listen on different ports in the same process. Register multiple services into one server instead.
