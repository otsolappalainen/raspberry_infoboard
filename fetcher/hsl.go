package fetcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"rasp_info/config"
	"rasp_info/store"
	"strings"
	"time"
)

const stopQueryFragment = `
  %s: stop(id: "%s") {
    name
    stoptimesWithoutPatterns(numberOfDepartures: 4) {
      scheduledDeparture
      realtimeDeparture
      departureDelay
      realtime
      realtimeState
      serviceDay
      headsign
      trip {
        route {
          shortName
        }
      }
    }
  }
`

type HSLFetcher struct {
	Config *config.Config
	Store  *store.Store
}

type StopResponse struct {
	Name      string `json:"name"`
	Stoptimes []struct {
		ScheduledDeparture int    `json:"scheduledDeparture"`
		RealtimeDeparture  int    `json:"realtimeDeparture"`
		Realtime           bool   `json:"realtime"`
		ServiceDay         int    `json:"serviceDay"`
		Headsign           string `json:"headsign"`
		Trip               struct {
			Route struct {
				ShortName string `json:"shortName"`
			} `json:"route"`
		} `json:"trip"`
	} `json:"stoptimesWithoutPatterns"`
}

// HSLResponse now maps dynamic keys to StopResponse
type HSLResponse struct {
	Data map[string]StopResponse `json:"data"`
}

func (f *HSLFetcher) Fetch() error {
	log.Println("Starting HSL fetch...")

	// Build dynamic query
	var queryBuilder bytes.Buffer
	queryBuilder.WriteString("{\n")

	// Map to store resolved IDs to original config index
	resolvedIDs := make(map[string]int)

	for i, stop := range f.Config.BusStops {
		stopID := stop.ID
		// Check if ID needs resolution (e.g. E1234-style code or plain code)
		if !strings.HasPrefix(stopID, "HSL:") {
			log.Printf("Resolving stop code via geocoding API: %s", stopID)
			id, err := f.LookupStop(stopID)
			if err != nil {
				log.Printf("Failed to resolve stop %s: %v", stopID, err)
				continue
			}
			log.Printf("Resolved %s to GTFS id %s", stopID, id)
			stopID = id
		}

		alias := fmt.Sprintf("stop%d", i)
		queryBuilder.WriteString(fmt.Sprintf(stopQueryFragment, alias, stopID))
		resolvedIDs[alias] = i
	}
	queryBuilder.WriteString("\n}")

	reqBody, _ := json.Marshal(map[string]string{
		"query": queryBuilder.String(),
	})

	req, err := http.NewRequest("POST", f.Config.HSLAPIUrl, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("HSL: Error creating request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("digitransit-subscription-key", f.Config.HSLKey)

	log.Printf("HSL: Sending request to %s", f.Config.HSLAPIUrl)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("HSL: Request failed: %v", err)
		return fmt.Errorf("failed to fetch HSL data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("HSL: API returned status %d", resp.StatusCode)
		return fmt.Errorf("HSL api returned status: %d", resp.StatusCode)
	}

	var result HSLResponse
	if err := json.Unmarshal(bodyBytes(resp.Body), &result); err != nil {
		log.Printf("HSL: Failed to decode JSON: %v", err)
		return fmt.Errorf("failed to decode HSL json: %w", err)
	}

	var stops []store.StopData

	// Iterate over resolved IDs to maintain order or just iterate config
	// We need to match the alias back to the config name

	// Create a map of alias -> StopResponse for easier access
	stopDataMap := result.Data
	log.Printf("HSL: Received data for %d stops", len(stopDataMap))

	for i, cfgStop := range f.Config.BusStops {
		alias := fmt.Sprintf("stop%d", i)
		if s, ok := stopDataMap[alias]; ok {
			var departures []store.Departure
			for _, st := range s.Stoptimes {
				departureTime := time.Unix(int64(st.ServiceDay)+int64(st.RealtimeDeparture), 0)
				departures = append(departures, store.Departure{
					RouteNumber: st.Trip.Route.ShortName,
					Destination: st.Headsign,
					Time:        departureTime,
					Realtime:    st.Realtime,
				})
			}
			stops = append(stops, store.StopData{
				StopName:   cfgStop.Name, // Use name from config
				Departures: departures,
			})
			log.Printf("HSL: Processed stop %s (%s): %d departures", cfgStop.Name, cfgStop.ID, len(departures))
		} else {
			log.Printf("HSL: No data found for stop alias %s (%s)", alias, cfgStop.Name)
		}
	}

	f.Store.UpdateTransport(store.TransportData{
		Stops:     stops,
		Timestamp: time.Now(),
	})

	log.Println("HSL: Fetch completed successfully")
	return nil
}

// Helper to read body without consuming it (for potential debugging, though here we just read it)
func bodyBytes(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

// LookupStop resolves human-friendly stop codes (e.g. E2185) into GTFS ids (HSL:xxxxx)
// using the Digitransit Pelias geocoding API.
func (f *HSLFetcher) LookupStop(shortCode string) (string, error) {
	// Build geocoding request URL
	url := fmt.Sprintf("https://api.digitransit.fi/geocoding/v1/search?text=%s&size=1&layers=stop&sources=gtfshsl", shortCode)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("digitransit-subscription-key", f.Config.HSLKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("geocoding api returned status: %d", resp.StatusCode)
	}

	var result struct {
		Features []struct {
			Properties struct {
				Gid string `json:"gid"`
			} `json:"properties"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Features) == 0 {
		return "", fmt.Errorf("no features found for code: %s", shortCode)
	}

	gid := result.Features[0].Properties.Gid
	// Example gid: gtfshsl:stop:GTFS:HSL:123455#E1234
	// We want to extract the GTFS stop id part: HSL:123455
	parts := strings.Split(gid, ":")
	if len(parts) < 5 {
		return "", fmt.Errorf("unexpected gid format: %s", gid)
	}
	gtfsStopIDWithExtra := parts[4] // e.g. 123455#E1234
	gtfsStopID := strings.Split(gtfsStopIDWithExtra, "#")[0]

	return fmt.Sprintf("HSL:%s", gtfsStopID), nil
}
