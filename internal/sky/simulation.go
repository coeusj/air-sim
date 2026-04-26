package sky

import (
	"context"
	"errors"
	"log"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/coeusj/air-sim/pkg/api/sim/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SkySimulation struct {
	sim.UnimplementedAircraftManagerServer
	provider    *DataProvider
	dataChannel chan *sim.Aircraft
	closer      sync.Once
}

func NewSkySimulation(provider *DataProvider) *SkySimulation {
	return &SkySimulation{
		provider:    provider,
		dataChannel: make(chan *sim.Aircraft),
	}
}

func (ss *SkySimulation) StreamAircraftList(request *sim.AircraftListRequest, stream sim.AircraftManager_StreamAircraftListServer) error {
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := ss.startSimulation(ctx, stream); err != nil {
				return err
			}
		}
	}
}

func (ss *SkySimulation) Close() {
	ss.closer.Do(func() {
		log.Println("[SkySimulation] Closing data channel")
		close(ss.dataChannel)
	})
}

func (ss *SkySimulation) startSimulation(ctx context.Context, stream sim.AircraftManager_StreamAircraftListServer) error {
	aircrafts, err := ss.provider.GetAircrafts(ctx)
	if err != nil {
		return err
	}
	if len(aircrafts) < 1 {
		return errors.New("No aircrafts found in DB")
	}

	routes, err := ss.provider.GetRoutes(ctx)
	if err != nil {
		return err
	}
	if len(routes) < 1 {
		return errors.New("No routes found in DB")
	}

	initializedAircrafts := initAircrafts(aircrafts)
	if err := stream.Send(initializedAircrafts); err != nil {
		return err
	}

	var wg sync.WaitGroup
	ss.initStreamSender(stream)
	ss.startSimulationUpdateLoop(ctx, &wg, aircrafts, routes)
	wg.Wait()

	log.Println("[SkySimulation] Simulation done")
	return nil
}

func initAircrafts(aircrafts []*Aircraft) *sim.AircraftListResponse {
	initialRes := sim.AircraftListResponse{}
	for _, aircraft := range aircrafts {
		initialRes.Aircrafts = append(initialRes.Aircrafts, &sim.Aircraft{
			Id:      strconv.Itoa(aircraft.Id),
			Lat:     float64(aircraft.Lat),
			Lon:     float64(aircraft.Lon),
			Alt:     float64(aircraft.Lon),
			Speed:   float64(aircraft.Speed),
			Heading: float64(aircraft.Heading),
			Track:   float64(aircraft.Track),
		})
	}

	return &initialRes
}

func (ss *SkySimulation) initStreamSender(stream sim.AircraftManager_StreamAircraftListServer) {
	go func() {
		streamRes := sim.AircraftListResponse{}

		for aircraft := range ss.dataChannel {
			streamRes.Aircrafts = append(streamRes.Aircrafts, aircraft)
			err := stream.Send(&streamRes)

			if err != nil {
				if stat, ok := status.FromError(err); ok {
					if stat.Code() == codes.Canceled || stat.Code() == codes.Unavailable {
						log.Printf("[SkySimulation] Client disconnect while sending data")
					}
				}

				log.Printf("[SkySimulation] Error while sending data: %s", err.Error())
				return
			}

			log.Printf("[SkySimulation] Aircrafts send successfully")
			streamRes.Aircrafts = nil
		}
	}()
}

func (ss *SkySimulation) startSimulationUpdateLoop(ctx context.Context, wg *sync.WaitGroup, aircrafts []*Aircraft, routes []*Route) {
	for _, aircraft := range aircrafts {
		var randLoopInterval uint32 = rand.Uint32N(5) + 1
		var loopCountMin uint32 = 70
		var loopCountMax uint32 = 150
		var randLoopCount uint32 = rand.Uint32N(loopCountMax-loopCountMin+1) + loopCountMin

		wg.Add(1)

		go func() {
			defer wg.Done()

			ticker := time.NewTicker(time.Second / time.Duration(randLoopInterval))
			defer ticker.Stop()

			for i := 0; i < int(randLoopCount); i++ {
				select {
				case <-ctx.Done():
					log.Printf("[SkySimulation] Simulation cancelled: %v\n", ctx.Err())
					return
				case <-ticker.C:
					frac := float32(i) / float32(randLoopCount)
					routeWithCoords := routes[aircraft.RouteId-1]
					aircraft.Lat = float32(routeWithCoords.StartLat) + (float32(routeWithCoords.EndLat)-float32(routeWithCoords.StartLat))*frac
					aircraft.Lon = float32(routeWithCoords.StartLon) + (float32(routeWithCoords.EndLon)-float32(routeWithCoords.StartLon))*frac

					ss.dataChannel <- &sim.Aircraft{
						Id:      strconv.Itoa(aircraft.Id),
						Lat:     float64(aircraft.Lat),
						Lon:     float64(aircraft.Lon),
						Alt:     float64(aircraft.Lon),
						Speed:   float64(aircraft.Speed),
						Heading: float64(aircraft.Heading),
						Track:   float64(aircraft.Track),
					}
				}
			}
		}()
	}
}
