package resolver

import (
	gResolver "google.golang.org/grpc/resolver"
)

const StaticScheme = "static"
const EtcdScheme = "etcd"

type IResolver interface {
	NewResolver() (gResolver.Builder, error)
}
