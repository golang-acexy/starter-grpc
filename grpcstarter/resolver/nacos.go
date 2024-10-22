package resolver

import (
	"errors"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	gResolver "google.golang.org/grpc/resolver"
)

type nacosBuilder struct {
	client naming_client.INamingClient
	group  string
}

func (n *nacosBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	if n.client == nil {
		return nil, errors.New("nacos client is nil")
	}
	r := &nacosResolver{
		client: n.client,
		target: target.Endpoint(),
		conn:   cc,
	}
	return r, nil
}

func (n *nacosBuilder) Scheme() string {
	return NacosScheme
}

type nacosResolver struct {
	client naming_client.INamingClient
	target string
	conn   gResolver.ClientConn
}

func (n *nacosResolver) ResolveNow(options gResolver.ResolveNowOptions) {
	panic("implement me")
}

func (n *nacosResolver) Close() {
	panic("implement me")
}

type Nacos struct {
	Client naming_client.INamingClient
	Group  string
}

func (n Nacos) NewResolver() (gResolver.Builder, error) {
	return &nacosBuilder{client: n.Client, group: n.Group}, nil
}
