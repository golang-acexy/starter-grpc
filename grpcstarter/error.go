package grpcstarter

import "errors"

var (
	// ErrGrpcServerAlreadyStarted reports another active lifecycle transition or server.
	ErrGrpcServerAlreadyStarted = errors.New("grpc server already started")
	// ErrGrpcServerNotStarted reports that no running server can be stopped.
	ErrGrpcServerNotStarted = errors.New("grpc server not started")
	// ErrGrpcStopTimeout reports forced shutdown after the requested grace period.
	ErrGrpcStopTimeout = errors.New("waiting for grpc server shutdown timeout")
)
