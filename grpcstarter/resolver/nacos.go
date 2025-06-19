package resolver

import (
	"context"
	"errors"
	"github.com/acexy/golang-toolkit/math/conversion"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	gResolver "google.golang.org/grpc/resolver"
)

type nacosBuilder struct {
	client      naming_client.INamingClient
	group       string
	watchCancel context.CancelFunc
	closeWatch  chan struct{}
}

func (n *nacosBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	if n.client == nil {
		return nil, errors.New("nacos client is nil")
	}
	r := &nacosResolver{
		client:  n.client,
		target:  target.Endpoint(),
		conn:    cc,
		builder: n,
	}
	instances, err := n.client.SelectInstances(vo.SelectInstancesParam{ServiceName: target.Endpoint(), GroupName: n.group, HealthyOnly: true})
	if err == nil && len(instances) > 0 {
		_ = r.conn.UpdateState(nacosInstanceToState(instances))
	} else {
		return nil, errors.New("no instance available")
	}
	ctx, cancel := context.WithCancel(context.Background())
	n.watchCancel = cancel
	n.closeWatch = make(chan struct{})
	go func() {
		// 异步监听变化
		param := &vo.SubscribeParam{ServiceName: target.Endpoint(), GroupName: n.group, SubscribeCallback: func(services []model.Instance, err error) {
			if err == nil && len(services) > 0 {
				_ = r.conn.UpdateState(nacosInstanceToState(services))
			}
		}}
		_ = n.client.Subscribe(param)
		select {
		case <-ctx.Done():
			_ = n.client.Unsubscribe(param)
			n.closeWatch <- struct{}{}
		}
	}()
	return r, nil
}

func nacosInstanceToState(instances []model.Instance) gResolver.State {
	resolverAddress := make([]gResolver.Address, len(instances))
	for i, instance := range instances {
		resolverAddress[i] = gResolver.Address{Addr: instance.Ip + ":" + conversion.FromUint64(instance.Port)}
	}
	return gResolver.State{Addresses: resolverAddress}
}

func (n *nacosBuilder) Scheme() string {
	return NacosScheme
}

type nacosResolver struct {
	client  naming_client.INamingClient
	target  string
	conn    gResolver.ClientConn
	builder *nacosBuilder
}

func (n *nacosResolver) ResolveNow(options gResolver.ResolveNowOptions) {
}

func (n *nacosResolver) Close() {
	n.builder.watchCancel()
	<-n.builder.closeWatch
}

type Nacos struct {
	client naming_client.INamingClient
	group  string
}

func NewNacosResolver(client naming_client.INamingClient, group string) *Nacos {
	return &Nacos{client: client, group: group}
}

func (n Nacos) NewResolver() (gResolver.Builder, error) {
	return &nacosBuilder{client: n.client, group: n.group}, nil
}
