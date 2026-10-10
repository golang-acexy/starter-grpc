package resolver

import "errors"

var (
	// ErrNacosClientNil reports an absent Nacos naming client.
	ErrNacosClientNil = errors.New("nacos client is nil")
	// ErrNoInstanceAvailable reports failed initial discovery or an empty healthy instance set.
	ErrNoInstanceAvailable = errors.New("no instance available")
	// ErrEtcdEndpointManager identifies failure to initialize Etcd endpoint management.
	ErrEtcdEndpointManager = errors.New("etcd resolver failed to create endpoint manager")
	// ErrEtcdWatchChannel identifies failure to establish an Etcd snapshot/watch.
	ErrEtcdWatchChannel = errors.New("etcd resolver failed to create watch channel")
)
