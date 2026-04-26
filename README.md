# gRPC
> protoc -I=api/sim/v1 --go_out=./pkg/api/sim/v1 --go_opt=paths=source_relative --go-grpc_out=./pkg/api/sim/v1 --go-grpc_opt=paths=source_relative api/sim/v1/aircraft.proto