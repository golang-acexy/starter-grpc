package resolver

import "errors"

var (
	ErrNacosClientNil      = errors.New("nacos client is nil")
	ErrNoInstanceAvailable = errors.New("no instance available")
	ErrEtcdEndpointManager = errors.New("etcd resolver failed to create endpoint manager")
	ErrEtcdWatchChannel    = errors.New("etcd resolver failed to create watch channel")
)
