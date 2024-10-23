package resolver

import (
	"context"
	etcdClient "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc/codes"
	gResolver "google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"sync"
)

type etcdBuilder struct {
	client *etcdClient.Client
}

func (b etcdBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &etcdResolver{
		client: b.client,
		target: target.Endpoint(),
		conn:   cc,
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	manager, err := endpoints.NewManager(r.client, r.target)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "etcdResolver: failed to new endpoint manager: %s", err)
	}

	all, err := manager.List(context.Background())
	if err == nil && len(all) > 0 {
		var addresses []gResolver.Address
		for _, v := range all {
			addresses = append(addresses, gResolver.Address{Addr: v.Addr})
		}
		_ = r.conn.UpdateState(gResolver.State{Addresses: addresses})
	}

	r.etcdWatch, err = manager.NewWatchChannel(r.ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "etcdResolver: failed to new watch channer: %s", err)
	}
	r.waitGroup.Add(1)
	go r.watch()
	return r, nil
}

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

func (r *etcdResolver) ResolveNow(gResolver.ResolveNowOptions) {}

func (r *etcdResolver) Close() {
	r.cancel()
	r.waitGroup.Wait()
	_ = r.client.Close()
}

type Etcd struct {
	client   *etcdClient.Client
	managers map[string]endpoints.Manager
}

func NewEtcdResolver(client *etcdClient.Client) *Etcd {
	return &Etcd{client: client}
}

func (e *Etcd) NewResolver() (gResolver.Builder, error) {
	e.managers = make(map[string]endpoints.Manager, 1)
	return &etcdBuilder{client: e.client}, nil
}
