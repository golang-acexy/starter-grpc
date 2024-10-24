package resolver

import (
	gResolver "google.golang.org/grpc/resolver"
)

// 静态gRPC服务端列表解析器
// 可以通过配置一批服务端列表，通过该解析器实现自动适配
type staticBuilder struct {
	addresses []string
	resolver  *StaticResolver
}

type StaticResolver struct {
	target    gResolver.Target
	cc        gResolver.ClientConn
	addresses []string
}

func (s *staticBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	r := &StaticResolver{
		target:    target,
		cc:        cc,
		addresses: s.addresses,
	}
	s.resolver = r
	r.register()
	return r, nil
}

func (*staticBuilder) Scheme() string { return StaticScheme }

func (r *StaticResolver) register() {
	_ = r.cc.UpdateState(staticAddressesToState(r.addresses))
}
func staticAddressesToState(addresses []string) gResolver.State {
	resolverAddress := make([]gResolver.Address, len(addresses))
	for i, address := range addresses {
		resolverAddress[i] = gResolver.Address{Addr: address}
	}
	return gResolver.State{Addresses: resolverAddress}
}

func (*StaticResolver) ResolveNow(o gResolver.ResolveNowOptions) {
}

func (*StaticResolver) Close() {}

type StaticResolverParam struct {
	Addresses map[string][]string
}

type Static struct {
	addresses []string
	builder   *staticBuilder
}

func NewStaticResolver(addresses []string) *Static {
	return &Static{addresses: addresses}
}

func (s *Static) NewResolver() (gResolver.Builder, error) {
	builder := &staticBuilder{addresses: s.addresses}
	s.builder = builder
	return builder, nil
}

// Update 更新地址信息
func (s *Static) Update(changed []string) {
	s.addresses = changed
	_ = s.builder.resolver.cc.UpdateState(staticAddressesToState(changed))
}
