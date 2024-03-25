# only for author's path
PROTO_BIN_PATH := /Users/acexy/Repository/cache/golang/project/demo-go-grpc/bin
export PATH := $(PROTO_BIN_PATH):${PATH}

default: test

test:
	protoc --go_out=. --go_opt=paths=source_relative \
        		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
        		./pbuser/user.proto \