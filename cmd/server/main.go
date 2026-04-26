package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/coeusj/air-sim/internal/sky"
	"github.com/coeusj/air-sim/pkg/api/sim/v1"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	if err := godotenv.Load("../../dev.env"); err != nil {
		log.Fatalf("Failed to load env file: %s\n", err.Error())
	}

	tcpServer, err := net.Listen(os.Getenv("TCP_LISTENER_NET_TYPE"), os.Getenv("TCP_LISTENER_HOST")+":"+os.Getenv("GRPC_PORT"))
	if err != nil {
		log.Fatalf("Error while creating tcp server: %s\n", err.Error())
	}
	defer tcpServer.Close()

	dataProvider := sky.NewDataProvider()
	defer dataProvider.Close()
	dataProvider.CreateAircrafts(context.Background(), 10)

	grpcServer := grpc.NewServer()
	// Single Server Approach (single gRPC server - multiple services) Monolithic
	// Bind the service servers struct to grpc server
	skySim := sky.NewSkySimulation(dataProvider)
	sim.RegisterAircraftManagerServer(grpcServer, skySim)
	defer skySim.Close()

	if err := grpcServer.Serve(tcpServer); err != nil {
		log.Fatalf("Could not start gRPC server: %s\n", err.Error())
	}

	log.Printf("gRPC server running on %s:%s\n", os.Getenv("TCP_LISTENER_HOST"), os.Getenv("GRPC_PORT"))
}
