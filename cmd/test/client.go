package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/coeusj/air-sim/pkg/api/sim/v1"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := godotenv.Load("../../dev.env"); err != nil {
		log.Fatalf("Failed to load env file: %s\n", err.Error())
	}

	connection, err := grpc.NewClient(os.Getenv("TCP_LISTENER_HOST")+":"+os.Getenv("GRPC_PORT"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Unable to connect to gRPC server")
		os.Exit(1)
	}
	defer connection.Close()

	trackerGRpcClient := sim.NewAircraftManagerClient(connection)
	res, err := trackerGRpcClient.StreamAircraftList(context.Background(), &sim.AircraftListRequest{})
	if err != nil {
		log.Fatal(err.Error())
	}

	for {
		data, err := res.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err.Error())
		}

		fmt.Println(data)
	}
}
