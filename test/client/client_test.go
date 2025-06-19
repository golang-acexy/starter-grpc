package client

import (
	"context"
	"fmt"
	"github.com/acexy/golang-toolkit/math/random"
	"github.com/acexy/golang-toolkit/util/json"
	"github.com/golang-acexy/starter-grpc/grpcstarter"
	"github.com/golang-acexy/starter-grpc/test/pbuser"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

var userService pbuser.UserServiceClient

func doRequest(ctx context.Context, gClient *grpcstarter.GrpcClient) {
	if userService == nil {
		userService = pbuser.NewUserServiceClient(gClient.GetConn())
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
	fmt.Println(json.ToJson(user))
}

// 使用直连的形式请求服务端
func TestCallServer(t *testing.T) {
	conn, err := grpcstarter.NewClientConn("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	doRequest(context.Background(), conn)
}
