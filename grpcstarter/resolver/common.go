package resolver

import (
	gResolver "google.golang.org/grpc/resolver"
)

// StaticScheme identifies targets resolved from a caller-supplied address list.
const StaticScheme = "static"
// EtcdScheme identifies targets resolved from an Etcd endpoint prefix.
const EtcdScheme = "etcd"
// NacosScheme identifies targets resolved through Nacos naming.
const NacosScheme = "nacos"

// IResolver supplies a per-connection resolver builder; target schemes must match.
type IResolver interface {
	// NewResolver creates a builder; remote discovery is deferred until gRPC invokes Build.
	NewResolver() (gResolver.Builder, error)
}
