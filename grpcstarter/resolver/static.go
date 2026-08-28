package resolver

import (
	"slices"
	"sync"

	"github.com/acexy/golang-toolkit/util/coll"
	gResolver "google.golang.org/grpc/resolver"
)

// 静态gRPC服务端列表解析器
// 可以通过配置一批服务端列表，通过该解析器实现自动适配
type staticBuilder struct {
	owner *Static
}

type StaticResolver struct {
	target       gResolver.Target
	cc           gResolver.ClientConn
	owner        *Static
	updateLock   sync.Mutex
	lastRevision uint64
	closed       bool
	closeOnce    sync.Once
}

func (s *staticBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &StaticResolver{target: target, cc: cc, owner: s.owner}
	addresses, revision := s.owner.addResolver(r)
	r.update(addresses, revision)
	return r, nil
}

func (*staticBuilder) Scheme() string { return StaticScheme }

func (r *StaticResolver) update(addresses []string, revision uint64) {
	r.updateLock.Lock()
	defer r.updateLock.Unlock()
	if r.closed || revision < r.lastRevision {
		return
	}
	r.lastRevision = revision
	_ = r.cc.UpdateState(staticAddressesToState(addresses))
}

func staticAddressesToState(addresses []string) gResolver.State {
	resolverAddress := coll.SliceCollect(addresses, func(address string) gResolver.Address {
		return gResolver.Address{Addr: address}
	})
	return gResolver.State{Addresses: resolverAddress}
}

func (*StaticResolver) ResolveNow(gResolver.ResolveNowOptions) {}

func (r *StaticResolver) Close() {
	r.closeOnce.Do(func() {
		r.owner.removeResolver(r)
		r.updateLock.Lock()
		r.closed = true
		r.updateLock.Unlock()
	})
}

type StaticResolverParam struct {
	Addresses map[string][]string
}

type Static struct {
	lock      sync.Mutex
	addresses []string
	resolvers map[*StaticResolver]struct{}
	revision  uint64
}

func NewStaticResolver(addresses []string) *Static {
	return &Static{addresses: slices.Clone(addresses), resolvers: make(map[*StaticResolver]struct{})}
}

func (s *Static) NewResolver() (gResolver.Builder, error) {
	return &staticBuilder{owner: s}, nil
}

func (s *Static) addResolver(resolver *StaticResolver) ([]string, uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.resolvers[resolver] = struct{}{}
	return slices.Clone(s.addresses), s.revision
}

func (s *Static) removeResolver(resolver *StaticResolver) {
	s.lock.Lock()
	delete(s.resolvers, resolver)
	s.lock.Unlock()
}

// Update 更新地址信息
func (s *Static) Update(changed []string) {
	s.lock.Lock()
	s.addresses = slices.Clone(changed)
	s.revision++
	revision := s.revision
	resolvers := coll.MapKeys(s.resolvers)
	addresses := slices.Clone(s.addresses)
	s.lock.Unlock()

	coll.SliceForEachAll(resolvers, func(resolver *StaticResolver) {
		resolver.update(addresses, revision)
	})
}
