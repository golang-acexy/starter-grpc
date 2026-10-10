package resolver

import (
	"errors"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	gResolver "google.golang.org/grpc/resolver"
)

type discoveryTestConn struct {
	staticTestClientConn
	errors []error
}

func (c *discoveryTestConn) ReportError(err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.errors = append(c.errors, err)
}

type nacosTestClient struct {
	naming_client.INamingClient
	instances    []model.Instance
	selectErr    error
	subscribeErr error
	cleanupErr   error
	subscribed   chan *vo.SubscribeParam
	cleaned      chan *vo.SubscribeParam
}

func (c *nacosTestClient) SelectInstances(vo.SelectInstancesParam) ([]model.Instance, error) {
	return c.instances, c.selectErr
}

func (c *nacosTestClient) Subscribe(param *vo.SubscribeParam) error {
	c.subscribed <- param
	return c.subscribeErr
}

func (c *nacosTestClient) Unsubscribe(param *vo.SubscribeParam) error {
	c.cleaned <- param
	return c.cleanupErr
}

func healthyTestInstance(ip string) model.Instance {
	return model.Instance{Ip: ip, Port: 8081, Healthy: true, Enable: true, Weight: 1}
}

func receiveSubscription(t *testing.T, ch <-chan *vo.SubscribeParam) *vo.SubscribeParam {
	t.Helper()
	select {
	case param := <-ch:
		return param
	case <-time.After(5 * time.Second):
		t.Fatal("subscription lifecycle did not complete")
		return nil
	}
}

func TestNacosUpdatesFilterAndWithdrawAddresses(t *testing.T) {
	client := &nacosTestClient{
		instances: []model.Instance{healthyTestInstance("127.0.0.1")},
		subscribed: make(chan *vo.SubscribeParam, 2), cleaned: make(chan *vo.SubscribeParam, 2),
	}
	builder, _ := NewNacosResolver(client, "group").NewResolver()
	conn := &discoveryTestConn{}
	r, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, conn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	param := receiveSubscription(t, client.subscribed)
	unhealthy := healthyTestInstance("127.0.0.2")
	unhealthy.Healthy = false
	disabled := healthyTestInstance("127.0.0.3")
	disabled.Enable = false
	zeroWeight := healthyTestInstance("127.0.0.4")
	zeroWeight.Weight = 0
	param.SubscribeCallback([]model.Instance{healthyTestInstance("2001:db8::1"), unhealthy, disabled, zeroWeight}, nil)
	if !reflect.DeepEqual(conn.addresses(), []string{"[2001:db8::1]:8081"}) {
		t.Fatalf("invalid healthy address set: %v", conn.addresses())
	}
	callbackErr := errors.New("update failed")
	param.SubscribeCallback(nil, callbackErr)
	if len(conn.addresses()) != 1 {
		t.Fatal("callback error should preserve the previous state")
	}
	param.SubscribeCallback([]model.Instance{unhealthy, disabled, zeroWeight}, nil)
	if len(conn.addresses()) != 0 {
		t.Fatal("all ineligible instances must withdraw the prior addresses")
	}
	param.SubscribeCallback([]model.Instance{healthyTestInstance("127.0.0.1")}, nil)
	param.SubscribeCallback(nil, nil)
	if len(conn.addresses()) != 0 {
		t.Fatal("empty update must withdraw the previous addresses")
	}
	r.Close()
	if receiveSubscription(t, client.cleaned) != param {
		t.Fatal("cleanup must use the exact original subscription pointer")
	}
	param.SubscribeCallback([]model.Instance{healthyTestInstance("127.0.0.1")}, nil)
	if len(conn.addresses()) != 0 {
		t.Fatal("closed resolver accepted a late callback")
	}
}

func TestNacosSubscriptionFailureAlwaysCleansUp(t *testing.T) {
	subscribeErr := errors.New("subscription failed")
	cleanupErr := errors.New("cleanup failed")
	client := &nacosTestClient{
		instances: []model.Instance{healthyTestInstance("127.0.0.1")},
		subscribeErr: subscribeErr, cleanupErr: cleanupErr,
		subscribed: make(chan *vo.SubscribeParam, 1), cleaned: make(chan *vo.SubscribeParam, 1),
	}
	builder, _ := NewNacosResolver(client, "group").NewResolver()
	conn := &discoveryTestConn{}
	r, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, conn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	if receiveSubscription(t, client.subscribed) != receiveSubscription(t, client.cleaned) {
		t.Fatal("failed subscription callback was not cleaned up by identity")
	}
	conn.lock.Lock()
	defer conn.lock.Unlock()
	if len(conn.errors) != 2 || !errors.Is(conn.errors[0], subscribeErr) || !errors.Is(conn.errors[1], cleanupErr) {
		t.Fatalf("subscription or cleanup error was lost: %v", conn.errors)
	}
}

func TestNacosInitialDiscoveryPreservesError(t *testing.T) {
	cause := errors.New("discovery failed")
	client := &nacosTestClient{selectErr: cause}
	builder, _ := NewNacosResolver(client, "").NewResolver()
	_, err := builder.Build(gResolver.Target{}, &discoveryTestConn{}, gResolver.BuildOptions{})
	if !errors.Is(err, cause) || !errors.Is(err, ErrNoInstanceAvailable) {
		t.Fatalf("initial discovery cause lost: %v", err)
	}
}

