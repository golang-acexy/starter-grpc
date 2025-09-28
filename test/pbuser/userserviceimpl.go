package pbuser

import (
	"context"

	"github.com/acexy/golang-toolkit/logger"
	"github.com/acexy/golang-toolkit/math/random"
	"google.golang.org/grpc/peer"
)

// 实现pb user的接口

type UserServiceImpl struct {
	UnimplementedUserServiceServer
}

func (u *UserServiceImpl) QueryById(ctx context.Context, request *Request) (*Response, error) {
	p, _ := peer.FromContext(ctx)
	logger.Logrus().Infoln(p.LocalAddr, "Get Input User", request.String())
	return &Response{
		Users: []*User{{
			Name: random.RandString(5),
		}},
	}, nil
}
