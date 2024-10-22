package resolver

import gResolver "google.golang.org/grpc/resolver"

type NacosResolver struct {
}

func (n *NacosResolver) ResolveNow(options gResolver.ResolveNowOptions) {
	//TODO implement me
	panic("implement me")
}

func (n *NacosResolver) Close() {
	//TODO implement me
	panic("implement me")
}

type Nacos struct {
	Addresses map[string][]string
}

func (n Nacos) NewResolver() (gResolver.Builder, error) {
	//TODO implement me
	panic("implement me")
}
