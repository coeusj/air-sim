package sky

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"os"
	"time"

	"github.com/coeusj/air-sim/pkg/utils"
	_ "github.com/microsoft/go-mssqldb"
)

type DataProvider struct {
	db *sql.DB
}

type Airport struct {
	Id          int
	Ident       string
	Name        string
	Lat         float32
	Lon         float32
	ElevationFt float32
	Continent   string
	Country     string
}

type Route struct {
	Id       int
	StartLat float32
	StartLon float32
	StartAlt float32
	EndLat   float32
	EndLon   float32
	EndAlt   float32
}

type Aircraft struct {
	Id      int
	Lat     float32
	Lon     float32
	Alt     float32
	Speed   float32
	Heading float32
	Track   float32
	RouteId int
}

func NewDataProvider() *DataProvider {
	dbUrl := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(os.Getenv("DB_USERNAME"), os.Getenv("Db_PASSWORD")),
		Host:   os.Getenv("DB_HOST"),
	}

	urlQueryParams := dbUrl.Query()
	urlQueryParams.Set("database", os.Getenv("DB_NAME"))
	urlQueryParams.Set("connection+timeout", "30")
	dbUrl.RawQuery = urlQueryParams.Encode()

	log.Println("[DataProvider] Connecting to SQLServer..")
	connString := dbUrl.String()
	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatalf("[DataProvider] Error while trying to open DB connection: %s", err.Error())
	}

	return &DataProvider{db: db}
}

func (dp *DataProvider) Close() {
	dp.db.Close()
}

func (dp *DataProvider) GetAirports(ctx context.Context) ([]*Airport, error) {
	query := `
		SELECT
			id,
			ident,
			name,
			latitude_deg,
			longitude_deg,
			elevation_ft,
			continent,
			iso_country
		FROM Airports
		WHERE continent = 'EU' AND type = 'large_airport';`

	rows, err := dp.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := []*Airport{}
	for rows.Next() {
		var id int
		var ident string
		var name string
		var latitude_deg float32
		var longitude_deg float32
		var elevation_ft float32
		var continent string
		var iso_country string

		if err := rows.Scan(&id, &ident, &name, &latitude_deg, &longitude_deg, &elevation_ft, &continent, &iso_country); err != nil {
			log.Printf("Error while scanning rows: %s", err.Error())
			continue
		}

		res = append(res, &Airport{
			Id:          id,
			Ident:       ident,
			Name:        name,
			Lat:         latitude_deg,
			Lon:         longitude_deg,
			ElevationFt: elevation_ft,
			Continent:   continent,
			Country:     iso_country,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

func (dp *DataProvider) GetAircrafts(ctx context.Context) ([]*Aircraft, error) {
	query := `
		SELECT
			id,
			lat,
			lon,
			alt,
			speed,
			heading,
			track,
			route_id
		FROM Aircrafts;`

	rows, err := dp.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := []*Aircraft{}
	for rows.Next() {
		var id int
		var lat float32
		var lon float32
		var alt float32
		var speed float32
		var heading float32
		var track float32
		var routeId int

		if err := rows.Scan(&id, &lat, &lon, &alt, &speed, &heading, &track, &routeId); err != nil {
			log.Printf("[DataProvider] Error while scanning rows: %s", err.Error())
			continue
		}

		res = append(res, &Aircraft{
			Id:      id,
			Lat:     lat,
			Lon:     lon,
			Alt:     alt,
			Speed:   speed,
			Heading: heading,
			Track:   track,
			RouteId: routeId,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

func (dp *DataProvider) TruncateTable(tableName string) error {
	query := fmt.Sprintf(`
		IF OBJECT_ID('%s', 'U') IS NOT NULL
		BEGIN
			TRUNCATE TABLE %s;
		END`, tableName, tableName)

	_, err := dp.db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

func (dp *DataProvider) CreateAircrafts(ctx context.Context, count int) {
	log.Println("[DataProvider] Trucate Aircrafts DB table")
	if err := dp.TruncateTable("Aircrafts"); err != nil {
		log.Fatalf("Could not truncate table: %s", err.Error())
	}

	routes, err := dp.GetRoutes(ctx)
	if err != nil {
		log.Fatalf("[DataProvider] Could not create routes")
	}

	log.Printf("[DataProvider] Creating aircrafts")
	insertQuery := ""
	for i := 0; i < count; i++ {
		randRouteIdx := rand.Uint32N(uint32(len(routes)))
		route := routes[randRouteIdx]

		aircraft := &Aircraft{
			Lat:     route.StartLat,
			Lon:     route.StartLon,
			Alt:     route.StartAlt,
			Speed:   0,
			Heading: 45.0,
			Track:   36.0,
			RouteId: route.Id,
		}

		insertQuery += fmt.Sprintf("INSERT INTO Aircrafts (lat, lon, alt, speed, heading, track, creation_date, route_id) VALUES (%.3f, %.3f, %.2f, %.2f, %.3f, %.3f, @p1, %d);",
			aircraft.Lat,
			aircraft.Lon,
			aircraft.Alt,
			aircraft.Speed,
			aircraft.Heading,
			aircraft.Track,
			aircraft.RouteId,
		)
	}

	_, err = dp.db.ExecContext(ctx, insertQuery, time.Now())
	if err != nil {
		log.Fatalf("[DataProvider] Error while on insert: %s", err.Error())
	}

	log.Println("[DataProvider] Aircrafts created")
}

func (dp *DataProvider) GetRoutes(ctx context.Context) ([]*Route, error) {
	query := `
		SELECT
			r.id,
			a.latitude_deg start_lat,
			a.longitude_deg start_lon,
			a.elevation_ft start_alt_ft,
			b.latitude_deg end_lat,
			b.longitude_deg end_lon,
			b.elevation_ft end_alt_ft
		FROM Routes r
		INNER JOIN Airports a ON r.start_id = a.id
		INNER JOIN Airports b ON r.end_id = b.id;`

	rows, err := dp.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := []*Route{}
	for rows.Next() {
		var id int
		var startLat float32
		var startLon float32
		var startAltFt float32
		var endLat float32
		var endLon float32
		var endAltFt float32

		if err := rows.Scan(&id, &startLat, &startLon, &startAltFt, &endLat, &endLon, &endAltFt); err != nil {
			log.Printf("[DataProvider] Error while scanning route: %s", err.Error())
			continue
		}

		res = append(res, &Route{
			Id:       id,
			StartLat: startLat,
			StartLon: startLon,
			StartAlt: utils.FeetToMeters(startAltFt),
			EndLat:   endLat,
			EndLon:   endLon,
			EndAlt:   utils.FeetToMeters(endAltFt),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}
