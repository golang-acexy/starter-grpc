package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/acexy/golang-toolkit/logger"
	"github.com/acexy/golang-toolkit/math/random"
	"github.com/acexy/golang-toolkit/sys"
	"github.com/acexy/golang-toolkit/util/json"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/test"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var userService pbuser.UserServiceClient

func doRequest(ctx context.Context, gClient *grpcstarter.GrpcClient) {
	if userService == nil {
		userService = pbuser.NewUserServiceClient(gClient.GetRawConn())
	}
	go func() {
		for {
			userCall(userService)
			time.Sleep(time.Second)
			select {
			case <-ctx.Done():
				_ = gClient.CloseConn()
				break
			default:
			}
		}
	}()
}

func userCall(userService pbuser.UserServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	user, err := userService.QueryById(ctx, &pbuser.Request{Id: uint64(random.RandInt(10))})
	if err != nil {
		statusError := status.Convert(err)
		fmt.Printf("%+v\n", statusError.Code())
		fmt.Printf("SelectById Error %T %+v\n", err, err)
		return
	}
	logger.Logrus().Infoln("request", json.ToString(user))
}

// 使用直连的形式请求服务端
func TestCallServer(t *testing.T) {
	logger.SetTraceIdSupplier(test.GetTraceIdSupplier())
	conn, err := grpcstarter.NewClientConnWithTraceSupplier(
		"localhost:8081",
		test.GetTraceIdSupplier(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	doRequest(context.Background(), conn)
	sys.ShutdownHolding()
}
