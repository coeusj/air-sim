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

const (
	minCommercialAlt float32 = 9000.00
	maxCommercialAlt float32 = 12800.00
	minLoop          uint32  = 70
	maxLoop          uint32  = 150
	stopAscentPerc   uint32  = 12
	startDescentPerc uint32  = 87
)

type SkySimulation struct {
	sim.UnimplementedAircraftManagerServer
	provider *DataProvider
	closer   sync.Once
}

func NewSkySimulation(provider *DataProvider) *SkySimulation {
	return &SkySimulation{
		provider: provider,
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

			return nil
		}
	}
}

func (ss *SkySimulation) Close() {
	ss.closer.Do(func() {
		log.Println("[SkySimulation] Simulation is closing")
	})
}

func (ss *SkySimulation) startSimulation(ctx context.Context, stream sim.AircraftManager_StreamAircraftListServer) error {
	log.Println("[SkySimulation] Simulation is starting")

	// TODO: set size if possible
	dataChannel := make(chan *sim.Aircraft)

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

	var updateLoopwg sync.WaitGroup
	senderWg := sync.WaitGroup{}

	senderWg.Add(1)
	go ss.initStreamSender(stream, dataChannel, &senderWg)

	ss.startSimulationUpdateLoop(ctx, &updateLoopwg, aircrafts, routes, dataChannel)

	updateLoopwg.Wait()
	close(dataChannel)

	senderWg.Wait()

	log.Println("[SkySimulation] Simulation completed")
	return nil
}

func initAircrafts(aircrafts []*Aircraft) *sim.AircraftListResponse {
	initialRes := sim.AircraftListResponse{}
	for _, aircraft := range aircrafts {
		initialRes.Aircrafts = append(initialRes.Aircrafts, &sim.Aircraft{
			Id:      strconv.Itoa(aircraft.Id),
			Lat:     aircraft.Lat,
			Lon:     aircraft.Lon,
			Alt:     aircraft.Alt,
			Speed:   aircraft.Speed,
			Heading: aircraft.Heading,
			Track:   aircraft.Track,
		})
	}

	return &initialRes
}

func (ss *SkySimulation) initStreamSender(stream sim.AircraftManager_StreamAircraftListServer, dataChannel chan *sim.Aircraft, senderWg *sync.WaitGroup) {
	defer senderWg.Done()
	streamRes := sim.AircraftListResponse{}

	for aircraft := range dataChannel {
		streamRes.Aircrafts = append(streamRes.Aircrafts, aircraft)
		err := stream.Send(&streamRes)

		if err != nil {
			if stat, ok := status.FromError(err); ok {
				if stat.Code() == codes.Canceled || stat.Code() == codes.Unavailable {
					log.Printf("[SkySimulation] Warning: Client disconnect while sending data")
				}
			}

			log.Printf("[SkySimulation] Error while sending data: %s", err.Error())
			return
		}

		log.Printf("[SkySimulation] Aircrafts sent successfully")
		streamRes.Aircrafts = nil
	}
}

func (ss *SkySimulation) startSimulationUpdateLoop(ctx context.Context, updateLoopWg *sync.WaitGroup, aircrafts []*Aircraft, routes []*Route, dataChannel chan *sim.Aircraft) {
	for _, aircraft := range aircrafts {
		updateLoopWg.Add(1)

		go func() {
			defer updateLoopWg.Done()

			route := routes[aircraft.RouteId-1]
			randLoopInterval := rand.Uint32N(5) + 1
			randLoopCount := rand.Uint32N(maxLoop-minLoop+1) + minLoop
			randTargetCommercialAlt := rand.Float32()*(maxCommercialAlt-minCommercialAlt) + minCommercialAlt
			ascentStop := (randLoopCount * stopAscentPerc) / 100
			descentStart := (randLoopCount * startDescentPerc) / 100

			ticker := time.NewTicker(time.Second / time.Duration(randLoopInterval))
			defer ticker.Stop()

			for i := 0; i < int(randLoopCount); i++ {
				select {
				case <-ctx.Done():
					log.Printf("[SkySimulation] Simulation cancelled: %v\n", ctx.Err())
					return
				case <-ticker.C:
					frac := float32(i) / float32(randLoopCount)
					aircraft.Lat = route.StartLat + (route.EndLat-route.StartLat)*frac
					aircraft.Lon = route.StartLon + (route.EndLon-route.StartLon)*frac

					if i < int(ascentStop) && aircraft.Alt < randTargetCommercialAlt {
						aircraft.Alt += (randTargetCommercialAlt - route.StartAlt) / float32(ascentStop)
					}

					if i >= int(descentStart) && aircraft.Alt > route.EndAlt {
						aircraft.Alt -= (randTargetCommercialAlt - route.EndAlt) / float32(randLoopCount-descentStart)
					}

					dataChannel <- &sim.Aircraft{
						Id:      strconv.Itoa(aircraft.Id),
						Lat:     aircraft.Lat,
						Lon:     aircraft.Lon,
						Alt:     aircraft.Alt,
						Speed:   aircraft.Speed,
						Heading: aircraft.Heading,
						Track:   aircraft.Track,
					}
				}
			}
		}()
	}
}
