package resolver

import (
	"context"
	"fmt"
	"sync"

	etcdClient "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc/codes"
	gResolver "google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
)

type etcdBuilder struct {
	client *etcdClient.Client
}

// Build establishes a snapshot-backed endpoint watch; client remains caller-owned.
func (b etcdBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &etcdResolver{
		client: b.client,
		target: target.Endpoint(),
		conn:   cc,
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	manager, err := endpoints.NewManager(r.client, r.target)
	if err != nil {
		r.cancel()
		return nil, status.Error(codes.InvalidArgument, fmt.Errorf("%w: %w", ErrEtcdEndpointManager, err).Error())
	}
	r.etcdWatch, err = manager.NewWatchChannel(r.ctx)
	if err != nil {
		r.cancel()
		return nil, status.Error(codes.Internal, fmt.Errorf("%w: %w", ErrEtcdWatchChannel, err).Error())
	}
	// The watch supplies its own snapshot; never publish an earlier independent List result.
	_ = r.conn.UpdateState(gResolver.State{})
	r.waitGroup.Add(1)
	go r.watch()
	return r, nil
}

// Scheme returns the Etcd target scheme.
func (b etcdBuilder) Scheme() string {
	return EtcdScheme
}

type etcdResolver struct {
	client    *etcdClient.Client
	target    string
	conn      gResolver.ClientConn
	etcdWatch endpoints.WatchChannel
	ctx       context.Context
	cancel    context.CancelFunc
	waitGroup sync.WaitGroup
	closeOnce sync.Once
}

func (r *etcdResolver) watch() {
	defer r.waitGroup.Done()
	allUps := make(map[string]*endpoints.Update)
	for {
		select {
		case <-r.ctx.Done():
			return
		case ups, ok := <-r.etcdWatch:
			if !ok {
				return
			}
			for _, up := range ups {
				switch up.Op {
				case endpoints.Add:
					allUps[up.Key] = up
				case endpoints.Delete:
					delete(allUps, up.Key)
				}
			}
			_ = r.conn.UpdateState(etcdAddressesToState(allUps))
		}
	}
}

func etcdAddressesToState(ups map[string]*endpoints.Update) gResolver.State {
	var addresses []gResolver.Address
	for _, up := range ups {
		addr := gResolver.Address{
			Addr: up.Endpoint.Addr,
		}
		addresses = append(addresses, addr)
	}
	return gResolver.State{Addresses: addresses}
}

// ResolveNow does nothing; endpoint updates are delivered by the watch.
func (r *etcdResolver) ResolveNow(gResolver.ResolveNowOptions) {}

// Close cancels this watch and waits for its processing loop; repeated calls are safe.
// The caller-owned Etcd client remains open.
func (r *etcdResolver) Close() {
	r.closeOnce.Do(func() {
		r.cancel()
		r.waitGroup.Wait()
	})
}

// Etcd builds independent endpoint watches using a shared caller-owned client.
type Etcd struct {
	client *etcdClient.Client
}

// NewEtcdResolver borrows client; close all gRPC connections before closing client.
// A nil client is rejected when the builder starts discovery.
func NewEtcdResolver(client *etcdClient.Client) *Etcd {
	return &Etcd{client: client}
}

// NewResolver returns a builder; discovery starts when gRPC activates the connection.
func (e *Etcd) NewResolver() (gResolver.Builder, error) {
	return &etcdBuilder{client: e.client}, nil
}
