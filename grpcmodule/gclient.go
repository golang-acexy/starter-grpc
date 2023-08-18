package grpcmodule

import (
	"github.com/acexy/golang-toolkit/log"
	"github.com/golang-acexy/starter-grpc/grpcmodule/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"time"
)

type Conn struct {
	gCon *grpc.ClientConn
}

func (g *Conn) GetConn() *grpc.ClientConn {
	return g.gCon
}

func (g *Conn) CloseConn() error {
	return g.gCon.Close()
}

func (g *Conn) IsConnReady() bool {
	state := g.GetConnState()
	if state != connectivity.Ready {
		return false
	}
	return true
}

func (g *Conn) GetConnState() connectivity.State {
	return g.gCon.GetState()
}

func checkConn(target string, c *Conn, waitConnReady bool, opts ...grpc.DialOption) (*Conn, error) {
	if waitConnReady {
		log.Logrus().Traceln("waitConnToReady = true, check conn state....")
		if !c.IsConnReady() {
			i := 0
			for !c.IsConnReady() {
				if i <= 10 {
					log.Logrus().Warningln("conn state not ready, still wait ...")
					if c.IsConnReady() {
						break
					}
				} else {
					_ = c.gCon.Close()
					log.Logrus().Warningln("conn state not ready, released old conn get new conn")
					conn, err := grpc.Dial(target, opts...)
					if err != nil {
						return nil, err
					}
					c.gCon = conn
					i = -1
				}
				i++
				time.Sleep(1 * time.Second)
			}
		}
	}
	log.Logrus().Traceln("waitConnToReady = true, conn ready to connect")
	return c, nil
}

// NewClientCoon 创建客户端连接
func NewClientCoon(target string, waitConnReady bool, opts ...grpc.DialOption) (*Conn, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	c := &Conn{
		gCon: conn,
	}
	return checkConn(target, c, waitConnReady, opts...)
}

// NewClientConnWithResolver 使用resolver配置服务端 创建客户端连接
func NewClientConnWithResolver(target string, iResolver resolver.IResolver, waitConnReady bool, opts ...grpc.DialOption) (*Conn, error) {
	gResolver, err := iResolver.NewResolver()
	if err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		opts = make([]grpc.DialOption, 1)
		opts[0] = grpc.WithResolvers(gResolver)
	} else {
		opts = append(opts, grpc.WithResolvers(gResolver))
	}
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	c := &Conn{
		gCon: conn,
	}
	return checkConn(target, c, waitConnReady, opts...)
}
