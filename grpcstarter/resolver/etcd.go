package resolver

import (
	"context"
	"github.com/acexy/golang-toolkit/logger"
	etcdClient "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
	"google.golang.org/grpc/codes"
	gResolver "google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"strings"
	"sync"
)

type etcdBuilder struct {
	c *etcdClient.Client
}

func (b etcdBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &resolver{
		c:      b.c,
		target: target.Endpoint(),
		cc:     cc,
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())

	em, err := endpoints.NewManager(r.c, r.target)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "resolver: failed to new endpoint manager: %s", err)
	}
	r.wch, err = em.NewWatchChannel(r.ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolver: failed to new watch channer: %s", err)
	}

	r.wg.Add(1)
	go r.watch()
	return r, nil
}

func (b etcdBuilder) Scheme() string {
	return EtcdScheme
}

type resolver struct {
	c      *etcdClient.Client
	target string
	cc     gResolver.ClientConn
	wch    endpoints.WatchChannel
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (r *resolver) watch() {
	defer r.wg.Done()
	allUps := make(map[string]*endpoints.Update)
	for {
		select {
		case <-r.ctx.Done():
			return
		case ups, ok := <-r.wch:
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
			addresses := convertToGRPCAddress(allUps)
			_ = r.cc.UpdateState(gResolver.State{Addresses: addresses})
		}
	}
}

func convertToGRPCAddress(ups map[string]*endpoints.Update) []gResolver.Address {
	var addresses []gResolver.Address
	for _, up := range ups {
		addr := gResolver.Address{
			Addr: up.Endpoint.Addr,
		}
		addresses = append(addresses, addr)
	}
	return addresses
}

func (r *resolver) ResolveNow(gResolver.ResolveNowOptions) {}

func (r *resolver) Close() {
	r.cancel()
	r.wg.Wait()
}

type Etcd struct {
	EtcdUrls []string
	etcd     *etcdClient.Client
	managers map[string]endpoints.Manager
}

func (e *Etcd) NewResolver() (gResolver.Builder, error) {
	var err error
	etcd, err := etcdClient.NewFromURLs(e.EtcdUrls)
	if err != nil {
		return nil, err
	}
	e.etcd = etcd
	e.managers = make(map[string]endpoints.Manager, 1)
	return &etcdBuilder{c: etcd}, nil
}

// RegisterEtcdSrvInstance 注册服务器实例 用于注册自身作为服务器实例，以便于其他客户端可以动态感知，
// ttl (s) 租约续期时间 如果在指定时间未续约，etcd将取消注册，其他客户端将无法感知该实例
func (e *Etcd) RegisterEtcdSrvInstance(ctx context.Context, target, instanceId, address string, ttl int64) {
	manager := e.managers[target]
	if manager == nil {
		var err error
		manager, err = endpoints.NewManager(e.etcd, target)
		if err != nil {
			logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
			return
		}
		e.managers[target] = manager
	}

	if strings.HasSuffix(target, "/") {
		target += instanceId
	} else {
		target += "/" + instanceId
	}

	lease, err := e.etcd.Grant(context.TODO(), ttl)
	if err != nil {
		logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
		return
	}
	err = manager.AddEndpoint(context.TODO(), target, endpoints.Endpoint{Addr: address}, etcdClient.WithLease(lease.ID))
	if err != nil {
		logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
		return
	}

	alive, err := e.etcd.KeepAlive(ctx, lease.ID)
	if err != nil {
		logger.Logrus().Errorln("etcd register manager target:", target, "address:", address, " error:", err)
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
			case <-alive:
				logger.Logrus().Traceln("auto keep alive grpc target:", target, "address:", address)
			}
		}
	}()
}
