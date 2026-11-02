package resolver

import (
	"errors"
	"net/url"
	"runtime"
	"sync"
	"testing"
	"time"
	"weak"

	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	etcdClient "go.etcd.io/etcd/client/v3"
	gResolver "google.golang.org/grpc/resolver"
)

func waitResolverPressure(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()
		runtime.GC()
		if check() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("closed resolvers remain retained")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStaticResolverPressure(t *testing.T) {
	owner := NewStaticResolver([]string{"127.0.0.1:8080"})
	builder, _ := owner.NewResolver()
	target := gResolver.Target{URL: url.URL{Path: "/service"}}
	refs := make([]weak.Pointer[StaticResolver], 0, 200)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			owner.Update([]string{"127.0.0.1:8080"})
		}
	}()
	for i := 0; i < 200; i++ {
		r, err := builder.Build(target, &staticTestClientConn{}, gResolver.BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		refs = append(refs, weak.Make(r.(*StaticResolver)))
		r.Close()
		r.Close()
	}
	wg.Wait()
	if len(owner.resolvers) != 0 {
		t.Fatal("closed static resolver still registered")
	}
	waitResolverPressure(t, func() bool {
		for _, ref := range refs {
			if ref.Value() != nil {
				return false
			}
		}
		return true
	})
}

func nacosPressureCycle(t *testing.T, fail bool) weak.Pointer[nacosResolver] {
	t.Helper()
	client := &nacosTestClient{instances: []model.Instance{healthyTestInstance("127.0.0.1")}, subscribed: make(chan *vo.SubscribeParam, 1), cleaned: make(chan *vo.SubscribeParam, 1)}
	if fail {
		client.subscribeErr = errors.New("injected subscription failure")
	}
	builder, _ := NewNacosResolver(client, "group").NewResolver()
	r, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, &discoveryTestConn{}, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	param := receiveSubscription(t, client.subscribed)
	r.Close()
	r.Close()
	cleaned := receiveSubscription(t, client.cleaned)
	if param != cleaned {
		t.Fatal("cleanup callback identity changed")
	}
	param.SubscribeCallback(nil, nil)
	return weak.Make(r.(*nacosResolver))
}
func TestNacosResolverPressure(t *testing.T) {
	refs := make([]weak.Pointer[nacosResolver], 0, 200)
	for i := 0; i < 200; i++ {
		refs = append(refs, nacosPressureCycle(t, i%2 == 0))
	}
	waitResolverPressure(t, func() bool {
		for _, ref := range refs {
			if ref.Value() != nil {
				return false
			}
		}
		return true
	})
}

func etcdPressureCycle(t *testing.T, client *etcdClient.Client) weak.Pointer[etcdResolver] {
	t.Helper()
	builder, _ := NewEtcdResolver(client).NewResolver()
	conn := &etcdTestConn{states: make(chan gResolver.State, 4)}
	r, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, conn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	receiveEtcdState(t, conn.states)
	r.Close()
	r.Close()
	if client.Ctx().Err() != nil {
		t.Fatal("borrowed client was closed")
	}
	return weak.Make(r.(*etcdResolver))
}
func TestEtcdResolverPressure(t *testing.T) {
	client, _, _ := newEtcdTestClient(t, &etcdClient.GetResponse{Header: &etcdserverpb.ResponseHeader{Revision: 10}}, nil)
	refs := make([]weak.Pointer[etcdResolver], 0, 200)
	for i := 0; i < 200; i++ {
		refs = append(refs, etcdPressureCycle(t, client))
	}
	waitResolverPressure(t, func() bool {
		for _, ref := range refs {
			if ref.Value() != nil {
				return false
			}
		}
		return true
	})
}
