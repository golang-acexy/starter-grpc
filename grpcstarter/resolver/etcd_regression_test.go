package resolver

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcdClient "go.etcd.io/etcd/client/v3"
	gResolver "google.golang.org/grpc/resolver"
)

type etcdTestKV struct {
	etcdClient.KV
	mu       sync.Mutex
	calls    int
	response *etcdClient.GetResponse
	err      error
}

func (kv *etcdTestKV) Get(context.Context, string, ...etcdClient.OpOption) (*etcdClient.GetResponse, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.calls++
	return kv.response, kv.err
}

type etcdTestWatcher struct {
	etcdClient.Watcher
	updates chan etcdClient.WatchResponse
}

func (w *etcdTestWatcher) Watch(context.Context, string, ...etcdClient.OpOption) etcdClient.WatchChan {
	return w.updates
}

func (*etcdTestWatcher) Close() error { return nil }

type etcdTestConn struct {
	discoveryTestConn
	states chan gResolver.State
}

func (c *etcdTestConn) UpdateState(state gResolver.State) error {
	_ = c.staticTestClientConn.UpdateState(state)
	c.states <- state
	return nil
}

func receiveEtcdState(t *testing.T, ch <-chan gResolver.State) gResolver.State {
	t.Helper()
	select {
	case state := <-ch:
		return state
	case <-time.After(5 * time.Second):
		t.Fatal("endpoint state was not delivered")
		return gResolver.State{}
	}
}

func newEtcdTestClient(t *testing.T, response *etcdClient.GetResponse, getErr error) (*etcdClient.Client, *etcdTestKV, *etcdTestWatcher) {
	t.Helper()
	client := etcdClient.NewCtxClient(context.Background())
	kv := &etcdTestKV{response: response, err: getErr}
	watcher := &etcdTestWatcher{updates: make(chan etcdClient.WatchResponse, 4)}
	client.KV = kv
	client.Watcher = watcher
	t.Cleanup(func() { _ = client.Close() })
	return client, kv, watcher
}

func TestEtcdSingleSnapshotAndCallerOwnership(t *testing.T) {
	response := &etcdClient.GetResponse{
		Header: &etcdserverpb.ResponseHeader{Revision: 10},
		Kvs: []*mvccpb.KeyValue{{Key: []byte("service/a"), Value: []byte(`{"Addr":"127.0.0.1:8081"}`)}},
	}
	client, kv, watcher := newEtcdTestClient(t, response, nil)
	builder, _ := NewEtcdResolver(client).NewResolver()
	conn := &etcdTestConn{states: make(chan gResolver.State, 8)}
	r, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, conn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if len(receiveEtcdState(t, conn.states).Addresses) != 0 {
		t.Fatal("builder published a separate potentially stale list")
	}
	initial := receiveEtcdState(t, conn.states)
	if len(initial.Addresses) != 1 || initial.Addresses[0].Addr != "127.0.0.1:8081" {
		t.Fatalf("watch snapshot was not applied: %+v", initial)
	}
	kv.mu.Lock()
	calls := kv.calls
	kv.mu.Unlock()
	if calls != 1 {
		t.Fatalf("expected one snapshot query, got %d", calls)
	}
	watcher.updates <- etcdClient.WatchResponse{Events: []*etcdClient.Event{{Type: mvccpb.DELETE, Kv: &mvccpb.KeyValue{Key: []byte("service/a")}}}}
	if len(receiveEtcdState(t, conn.states).Addresses) != 0 {
		t.Fatal("deletion did not withdraw the snapshot address")
	}
	r.Close()
	r.Close()
	if client.Ctx().Err() != nil {
		t.Fatal("closing a resolver closed the caller-owned etcd client")
	}
	secondConn := &etcdTestConn{states: make(chan gResolver.State, 8)}
	second, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, secondConn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatalf("shared client could not build another resolver: %v", err)
	}
	second.Close()
}

func TestEtcdEmptySnapshotClearsPriorState(t *testing.T) {
	client, _, watcher := newEtcdTestClient(t, &etcdClient.GetResponse{Header: &etcdserverpb.ResponseHeader{Revision: 10}}, nil)
	builder, _ := NewEtcdResolver(client).NewResolver()
	conn := &etcdTestConn{states: make(chan gResolver.State, 8)}
	_ = conn.staticTestClientConn.UpdateState(gResolver.State{Addresses: []gResolver.Address{{Addr: "stale:8081"}}})
	r, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, conn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if len(receiveEtcdState(t, conn.states).Addresses) != 0 {
		t.Fatal("empty snapshot left prior addresses available")
	}
	watcher.updates <- etcdClient.WatchResponse{Events: []*etcdClient.Event{{Type: mvccpb.PUT, Kv: &mvccpb.KeyValue{Key: []byte("service/b"), Value: []byte(`{"Addr":"127.0.0.1:8082"}`)}}}}
	_ = receiveEtcdState(t, conn.states)
	if !reflect.DeepEqual(conn.addresses(), []string{"127.0.0.1:8082"}) {
		t.Fatalf("update after empty snapshot failed: %v", conn.addresses())
	}
}

func TestEtcdWatchSetupFailurePreservesClient(t *testing.T) {
	client, _, _ := newEtcdTestClient(t, nil, errors.New("snapshot failed"))
	builder, _ := NewEtcdResolver(client).NewResolver()
	_, err := builder.Build(gResolver.Target{URL: url.URL{Path: "/service"}}, &etcdTestConn{}, gResolver.BuildOptions{})
	if err == nil || client.Ctx().Err() != nil {
		t.Fatal("setup failure must report an error without closing the shared client")
	}
}
