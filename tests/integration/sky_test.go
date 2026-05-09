//go:build integration
// +build integration

package tests

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/coeusj/air-sim/internal/sky"
	"github.com/coeusj/air-sim/pkg/api/sim/v1"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize = 1024 * 1024
)

var listener *bufconn.Listener
var dataProvider *sky.DataProvider
var ctx context.Context

func TestMain(t *testing.M) {
	if err := godotenv.Load("../../test.env"); err != nil {
		log.Fatalf("Failed to load env file: %s\n", err.Error())
	}

	ctx = context.Background()
	listener = bufconn.Listen(bufSize)
	server := grpc.NewServer()
	dataProvider = sky.NewDataProvider(os.Getenv("DB_HOST"), os.Getenv("DB_NAME"), os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD"))

	sim.RegisterAircraftManagerServer(server, sky.NewSkySimulation(dataProvider))
	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	// Give a moment to initialize the DB connection
	time.Sleep(time.Second)

	code := t.Run()
	os.Exit(code)
}

func TestProviderGetAirPorts(t *testing.T) {
	airports, err := dataProvider.GetAirports(ctx)
	if err != nil {
		t.Errorf("Failed to get airports: %v", err)
	}

	if len(airports) <= 0 {
		t.Error("No airports found")
	}
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return listener.Dial()
}

func TestAircraftsNavigationDataStream(t *testing.T) {
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := sim.NewAircraftManagerClient(conn)
	stream, err := client.StreamAircraftList(ctx, &sim.AircraftListRequest{})
	if err != nil {
		t.Fatalf("Open stream failed: %v", err)
	}

	var count int
	for {
		_, err = stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Receive failed: %v", err)
		}

		count++
	}

	if count == 0 {
		t.Error("No aircraft received")
	}
}
