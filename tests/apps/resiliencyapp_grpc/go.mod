module github.com/dapr/dapr/tests/apps/resiliencyapp_grpc

go 1.21

toolchain go1.21.4

require (
	github.com/dapr/dapr v1.7.4
	google.golang.org/grpc v1.57.1
	google.golang.org/grpc/examples v0.0.0-20220818173707-97cb7b1653d7
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
)

replace github.com/dapr/dapr => ../../../
