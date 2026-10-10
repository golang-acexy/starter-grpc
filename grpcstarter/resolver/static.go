package resolver

import (
	"slices"
	"sync"

	"github.com/acexy/golang-toolkit/util/coll"
	gResolver "google.golang.org/grpc/resolver"
)

// staticBuilder connects one gRPC connection to the shared static address owner.
type staticBuilder struct {
	owner *Static
}

// StaticResolver delivers static address revisions to one connection.
// Instances are builder-owned; Close is idempotent and detaches only this connection.
type StaticResolver struct {
	target       gResolver.Target
	cc           gResolver.ClientConn
	owner        *Static
	updateLock   sync.Mutex
	lastRevision uint64
	closed       bool
	closeOnce    sync.Once
}

// Build attaches this connection to the shared address list and publishes its current state.
func (s *staticBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &StaticResolver{target: target, cc: cc, owner: s.owner}
	addresses, revision := s.owner.addResolver(r)
	r.update(addresses, revision)
	return r, nil
}

// Scheme returns the static target scheme.
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

// ResolveNow does nothing; the owner supplies updates through Static.Update.
func (*StaticResolver) ResolveNow(gResolver.ResolveNowOptions) {}

// Close detaches this resolver without affecting other connections; repeated calls are safe.
func (r *StaticResolver) Close() {
	r.closeOnce.Do(func() {
		r.owner.removeResolver(r)
		r.updateLock.Lock()
		r.closed = true
		r.updateLock.Unlock()
	})
}

// StaticResolverParam holds service-specific static address lists; current constructors use a slice.
type StaticResolverParam struct {
	// Addresses maps service names to endpoint lists.
	Addresses map[string][]string
}

// Static owns an address list and fans out revisions to all active connections.
// Use NewStaticResolver; Update and builder attachment are synchronized.
type Static struct {
	lock      sync.Mutex
	addresses []string
	resolvers map[*StaticResolver]struct{}
	revision  uint64
}

// NewStaticResolver copies addresses; an empty list provides no endpoints.
// Callers select credentials and load-balancing policy through gRPC dial options.
func NewStaticResolver(addresses []string) *Static {
	return &Static{addresses: slices.Clone(addresses), resolvers: make(map[*StaticResolver]struct{})}
}

// NewResolver creates a builder sharing this owner while retaining per-connection closure.
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

// Update copies and publishes a replacement list to all active connections.
// An empty list withdraws endpoints; this method does not perform health checks.
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
