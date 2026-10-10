package resolver

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	gResolver "google.golang.org/grpc/resolver"
)

type nacosBuilder struct {
	client naming_client.INamingClient
	group  string
}

// Build resolves the initial healthy set and starts an independent SDK subscription.
// The supplied naming client remains caller-owned; initial discovery must be nonempty.
func (n *nacosBuilder) Build(target gResolver.Target, cc gResolver.ClientConn, opts gResolver.BuildOptions) (gResolver.Resolver, error) {
	if n.client == nil {
		return nil, ErrNacosClientNil
	}
	r := &nacosResolver{
		client: n.client,
		target: target.Endpoint(),
		group:  n.group,
		conn:   cc,
	}
	instances, err := n.client.SelectInstances(vo.SelectInstancesParam{ServiceName: target.Endpoint(), GroupName: n.group, HealthyOnly: true})
	if err != nil {
		return nil, errors.Join(ErrNoInstanceAvailable, err)
	}
	initial := nacosInstanceToState(instances)
	if len(initial.Addresses) == 0 {
		return nil, ErrNoInstanceAvailable
	}
	_ = r.conn.UpdateState(initial)
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		param := &vo.SubscribeParam{ServiceName: r.target, GroupName: r.group, SubscribeCallback: r.update}
		// The SDK retains a callback even when its remote subscription attempt fails.
		defer func() {
			if err := n.client.Unsubscribe(param); err != nil {
				r.conn.ReportError(err)
			}
		}()
		if err := n.client.Subscribe(param); err != nil {
			r.conn.ReportError(err)
			return
		}
		<-ctx.Done()
	}()
	return r, nil
}

func nacosInstanceToState(instances []model.Instance) gResolver.State {
	addresses := make([]gResolver.Address, 0, len(instances))
	for _, instance := range instances {
		if !instance.Healthy || !instance.Enable || instance.Weight <= 0 {
			continue
		}
		addresses = append(addresses, gResolver.Address{Addr: net.JoinHostPort(instance.Ip, strconv.FormatUint(instance.Port, 10))})
	}
	return gResolver.State{Addresses: addresses}
}

func (n *nacosResolver) update(instances []model.Instance, err error) {
	n.updateLock.Lock()
	defer n.updateLock.Unlock()
	if n.closed {
		return
	}
	if err != nil {
		n.conn.ReportError(err)
		return
	}
	// An empty healthy set must withdraw all previously published addresses.
	_ = n.conn.UpdateState(nacosInstanceToState(instances))
}

// Scheme returns the Nacos target scheme.
func (n *nacosBuilder) Scheme() string {
	return NacosScheme
}

type nacosResolver struct {
	client     naming_client.INamingClient
	target     string
	group      string
	conn       gResolver.ClientConn
	cancel     context.CancelFunc
	done       chan struct{}
	closeOnce  sync.Once
	updateLock sync.Mutex
	closed     bool
}

// ResolveNow does nothing; the SDK subscription supplies updates.
func (n *nacosResolver) ResolveNow(options gResolver.ResolveNowOptions) {}

// Close stops this subscription and waits for cleanup without closing the naming client.
func (n *nacosResolver) Close() {
	n.closeOnce.Do(func() {
		n.updateLock.Lock()
		n.closed = true
		n.updateLock.Unlock()
		n.cancel()
		<-n.done
	})
}

// Nacos constructs independent resolvers using a shared caller-owned SDK client.
type Nacos struct {
	client naming_client.INamingClient
	group  string
}

// NewNacosResolver uses client for discovery; group defaults to DEFAULT_GROUP when empty.
// Enable SDK UpdateCacheWhenEmpty to receive notifications when the last instance disappears.
func NewNacosResolver(client naming_client.INamingClient, group string) *Nacos {
	return &Nacos{client: client, group: group}
}

// NewResolver returns a builder; nil clients and initial discovery errors are checked during Build.
func (n Nacos) NewResolver() (gResolver.Builder, error) {
	return &nacosBuilder{client: n.client, group: n.group}, nil
}
