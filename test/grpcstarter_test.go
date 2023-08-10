package test

import (
	"github.com/golang-acexy/starter-grpc/grpcmodule"
	"github.com/golang-acexy/starter-parent/parentmodule/declaration"
	"testing"
	"time"
)

var moduleLoaders []declaration.ModuleLoader
var gModule *grpcmodule.GrpcModule

func init() {
	gModule = &grpcmodule.GrpcModule{}
	moduleLoaders = []declaration.ModuleLoader{gModule}
}

func TestLoadAndUnload(t *testing.T) {
	m := declaration.Module{
		ModuleLoaders: moduleLoaders,
	}
	m.Load()
	time.Sleep(time.Second * 2)
	m.Unload(10)
}
