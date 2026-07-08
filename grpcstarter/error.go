package grpcstarter

import "errors"

var (
	ErrGrpcServerAlreadyStarted = errors.New("grpc server already started")
	ErrGrpcServerNotStarted     = errors.New("grpc server not started")
	ErrGrpcStopTimeout          = errors.New("waiting for grpc server shutdown timeout")
)
