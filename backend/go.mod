module github.com/bayu-aditya/data-platform/server

go 1.26.2

require (
	connectrpc.com/connect v1.20.0
	connectrpc.com/grpchealth v1.5.0
)

require google.golang.org/protobuf v1.36.12 // indirect

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	google.golang.org/protobuf/cmd/protoc-gen-go
)
