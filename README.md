# Air Traffic Simulator

Simulating aircrafts and send telemetry data via gRPC

## Build

Compile proto files:

```bash
protoc -I=api/sim/v1 --go_out=./pkg/api/sim/v1 --go_opt=paths=source_relative --go-grpc_out=./pkg/api/sim/v1 --go-grpc_opt=paths=source_relative api/sim/v1/aircraft.proto
```

Environment varibles for development and testing purpose:

- `.\dev.env`
- `.\test.env`

## Run

To run the application:

```bash
go run cmd/server/main.go
```

## Tests

To run the tests:

```bash
go test -v --tags=integration -run <test-method> ./tests/integration/
```