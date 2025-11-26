package fetcher

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"rasp_info/config"
	"rasp_info/store"
	"strings"
	"time"
)

type FMIFetcher struct {
	Config *config.Config
	Store  *store.Store
}

// XML Structures for parsing FMI WFS response (simplified)
type FeatureCollection struct {
	Member []Member `xml:"member"`
}

type Member struct {
	PointTimeSeriesObservation PointTimeSeriesObservation `xml:"PointTimeSeriesObservation"`
}

type PointTimeSeriesObservation struct {
	Result Result `xml:"result"`
}

type Result struct {
	MeasurementTimeseries MeasurementTimeseries `xml:"MeasurementTimeseries"`
}

type MeasurementTimeseries struct {
	ID    string           `xml:"id,attr"` // To distinguish between parameters
	Point []MeasurementTVP `xml:"point"`
}

type MeasurementTVP struct {
	MeasurementTVP struct {
		Time  string  `xml:"time"`
		Value float64 `xml:"value"`
	} `xml:"MeasurementTVP"`
}

func (f *FMIFetcher) Fetch() error {
	// Build URL
	// https://opendata.fmi.fi/wfs?service=WFS&version=2.0.0&request=getFeature&storedquery_id=fmi::forecast::harmonie::surface::point::timevaluepair&place=Espoo&timestep=60&parameters=temperature,Precipitation1h&starttime=...&endtime=...

	now := time.Now().UTC()
	endTime := now.Add(24 * time.Hour)

	baseURL, _ := url.Parse(f.Config.FMIAPIUrl)
	params := url.Values{}
	params.Add("service", "WFS")
	params.Add("version", "2.0.0")
	params.Add("request", "getFeature")
	params.Add("storedquery_id", "fmi::forecast::harmonie::surface::point::timevaluepair")
	params.Add("place", f.Config.WeatherLocation)
	params.Add("timestep", "60")
	params.Add("parameters", "temperature,Precipitation1h,Pop")
	params.Add("starttime", now.Format(time.RFC3339))
	params.Add("endtime", endTime.Format(time.RFC3339))

	baseURL.RawQuery = params.Encode()

	resp, err := http.Get(baseURL.String())
	if err != nil {
		return fmt.Errorf("failed to fetch FMI data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("FMI api returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read FMI body: %w", err)
	}

	// Log raw XML for debugging
	// log.Printf("FMI Response: %s", string(bodyBytes))

	var collection FeatureCollection
	if err := xml.Unmarshal(bodyBytes, &collection); err != nil {
		return fmt.Errorf("failed to decode FMI XML: %w", err)
	}

	// Process data
	var forecast []store.WeatherDataPoint
	var current store.WeatherDataPoint

	// Maps to hold data by time
	temps := make(map[time.Time]float64)
	precips := make(map[time.Time]float64)
	pops := make(map[time.Time]float64)
	times := make(map[time.Time]bool)

	for _, member := range collection.Member {
		ts := member.PointTimeSeriesObservation.Result.MeasurementTimeseries

		if strings.Contains(ts.ID, "temperature") {
			for _, p := range ts.Point {
				t, err := time.Parse(time.RFC3339, p.MeasurementTVP.Time)
				if err != nil {
					continue
				}
				temps[t] = p.MeasurementTVP.Value
				times[t] = true
			}
		} else if strings.Contains(ts.ID, "Precipitation1h") {
			for _, p := range ts.Point {
				t, err := time.Parse(time.RFC3339, p.MeasurementTVP.Time)
				if err != nil {
					continue
				}
				precips[t] = p.MeasurementTVP.Value
				times[t] = true
			}
		} else if strings.Contains(ts.ID, "Pop") {
			for _, p := range ts.Point {
				t, err := time.Parse(time.RFC3339, p.MeasurementTVP.Time)
				if err != nil {
					continue
				}
				pops[t] = p.MeasurementTVP.Value
				times[t] = true
			}
		}
	}

	// Combine data
	for t := range times {
		temp, okT := temps[t]
		precip, okP := precips[t]
		pop, okPop := pops[t]

		if !okT || math.IsNaN(temp) {
			temp = 0
		}
		if !okP || math.IsNaN(precip) {
			precip = 0
		}
		if !okPop || math.IsNaN(pop) {
			pop = 0
		}

		// Determine symbol
		symbol := "cloudy"
		if precip > 0.1 || pop > 50 {
			symbol = "rain"
		}

		wp := store.WeatherDataPoint{
			Temperature:   temp,
			Precipitation: precip,
			Pop:           pop,
			Symbol:        symbol,
			Time:          t,
		}

		forecast = append(forecast, wp)

		// Find current (closest to now)
		if current.Time.IsZero() || absDiff(t, now) < absDiff(current.Time, now) {
			current = wp
		}
	}

	f.Store.UpdateWeather(store.WeatherData{
		Current:  current,
		Forecast: forecast,
	})

	return nil
}

func absDiff(a, b time.Time) time.Duration {
	d := a.Sub(b)
	if d < 0 {
		return -d
	}
	return d
}
