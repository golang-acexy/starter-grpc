package resolver

import (
	"reflect"
	"sync"
	"testing"

	gResolver "google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

type staticTestClientConn struct {
	lock  sync.Mutex
	state gResolver.State
}

func (c *staticTestClientConn) UpdateState(state gResolver.State) error {
	c.lock.Lock()
	c.state = state
	c.lock.Unlock()
	return nil
}

func (*staticTestClientConn) ReportError(error) {}
func (*staticTestClientConn) NewAddress([]gResolver.Address) {}
func (*staticTestClientConn) ParseServiceConfig(string) *serviceconfig.ParseResult { return nil }

func (c *staticTestClientConn) addresses() []string {
	c.lock.Lock()
	defer c.lock.Unlock()
	result := make([]string, len(c.state.Addresses))
	for index, address := range c.state.Addresses {
		result[index] = address.Addr
	}
	return result
}

func TestStaticResolverUpdatesAllActiveResolvers(t *testing.T) {
	input := []string{"127.0.0.1:8081"}
	static := NewStaticResolver(input)
	input[0] = "changed"
	builder, err := static.NewResolver()
	if err != nil {
		t.Fatal(err)
	}

	firstConn := &staticTestClientConn{}
	first, err := builder.Build(gResolver.Target{}, firstConn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	secondConn := &staticTestClientConn{}
	second, err := builder.Build(gResolver.Target{}, secondConn, gResolver.BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firstConn.addresses(), []string{"127.0.0.1:8081"}) {
		t.Fatalf("构造参数未被复制: %v", firstConn.addresses())
	}

	static.Update([]string{"127.0.0.1:8082"})
	if !reflect.DeepEqual(firstConn.addresses(), []string{"127.0.0.1:8082"}) || !reflect.DeepEqual(secondConn.addresses(), []string{"127.0.0.1:8082"}) {
		t.Fatal("地址更新未同步到全部 resolver")
	}

	first.Close()
	static.Update([]string{"127.0.0.1:8083"})
	if !reflect.DeepEqual(firstConn.addresses(), []string{"127.0.0.1:8082"}) {
		t.Fatal("已关闭 resolver 不应继续接收更新")
	}
	if !reflect.DeepEqual(secondConn.addresses(), []string{"127.0.0.1:8083"}) {
		t.Fatal("活动 resolver 未收到最新地址")
	}
	second.Close()
}
