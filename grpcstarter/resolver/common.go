package resolver

import (
	gResolver "google.golang.org/grpc/resolver"
)

const StaticScheme = "static"
const EtcdScheme = "etcd"
const NacosScheme = "nacos"

type IResolver interface {
	NewResolver() (gResolver.Builder, error)
}
