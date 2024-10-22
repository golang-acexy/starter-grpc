package resolver

import (
	gResolver "google.golang.org/grpc/resolver"
)

// 静态gRPC服务端列表解析器
// 可以通过配置一批服务端列表，通过该解析器实现自动适配
type staticBuilder struct {
	addresses map[string][]string
}

type StaticResolver struct {
	target    gResolver.Target
	cc        gResolver.ClientConn
	addresses map[string][]string
}

func (b *staticBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &StaticResolver{
		target:    target,
		cc:        cc,
		addresses: b.addresses,
	}
	r.register()
	return r, nil
}

func (*staticBuilder) Scheme() string { return StaticScheme }

func (r *StaticResolver) register() {
	addresses := r.addresses[r.target.Endpoint()]
	resolverAddress := make([]gResolver.Address, len(addresses))
	for i, address := range addresses {
		resolverAddress[i] = gResolver.Address{Addr: address}
	}
	err := r.cc.UpdateState(gResolver.State{Addresses: resolverAddress})
	if err != nil {
		return
	}
}

func (*StaticResolver) ResolveNow(o gResolver.ResolveNowOptions) {
}

func (*StaticResolver) Close() {}

type StaticResolverParam struct {
	Addresses map[string][]string
}

type Static struct {
	Addresses map[string][]string
}

func (s Static) NewResolver() (gResolver.Builder, error) {
	return &staticBuilder{addresses: s.Addresses}, nil
}
