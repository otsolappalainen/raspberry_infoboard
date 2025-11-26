package config

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Port string `json:"port"`

	// Fetch Intervals
	WeatherInterval     time.Duration `json:"-"`
	TransportInterval   time.Duration `json:"-"`
	ElectricityInterval time.Duration `json:"-"`

	// API Keys and URLs
	HSLAPIUrl  string `json:"hsl_api_url"`
	HSLKey     string `json:"hsl_api_key"`
	FMIAPIUrl  string `json:"fmi_api_url"`
	SpotAPIUrl string `json:"spot_api_url"`

	// User Settings
	WeatherLocation string    `json:"weather_location"`
	BusStops        []BusStop `json:"bus_stops"`
}

type BusStop struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Load returns a configuration, reading from config.json if available
func Load() *Config {
	cfg := &Config{
		Port:                ":8080",
		WeatherInterval:     15 * time.Minute,
		TransportInterval:   5 * time.Minute,
		ElectricityInterval: 15 * time.Minute,
		HSLAPIUrl:           "https://api.digitransit.fi/routing/v2/hsl/gtfs/v1",
		FMIAPIUrl:           "https://opendata.fmi.fi/wfs",
		SpotAPIUrl:          "https://api.spot-hinta.fi/TodayAndDayForward?region=FI&priceResolution=15",
		WeatherLocation:     "Espoo",     // Default
		BusStops:            []BusStop{}, // No defaults - user must configure
	}

	// Try loading from config.json
	if data, err := os.ReadFile("config.json"); err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			log.Printf("Error parsing config.json: %v", err)
		}
	} else {
		// Fallback to secrets.txt for HSL key if config.json not found
		if data, err := os.ReadFile("secrets.txt"); err == nil {
			cfg.HSLKey = strings.TrimSpace(string(data))
		} else {
			log.Println("Warning: Could not read config.json or secrets.txt")
		}
	}

	return cfg
}
