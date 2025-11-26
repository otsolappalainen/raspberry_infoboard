package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"rasp_info/config"
	"rasp_info/fetcher"
	"rasp_info/store"
	"runtime"
	"time"
)

var startTime = time.Now()

func main() {
	// Parse flags
	lookupCode := flag.String("lookup", "", "Lookup HSL stop by short code (e.g. E2185)")
	flag.Parse()

	cfg := config.Load()
	st := store.New()

	// Setup Log Capture
	logWriter := &LogWriter{
		Target: os.Stdout,
		Store:  st,
	}
	log.SetOutput(logWriter)

	// Initialize Fetchers
	hslFetcher := &fetcher.LoggingFetcher{
		Fetcher: &fetcher.HSLFetcher{Config: cfg, Store: st},
		Store:   st,
		Name:    "HSL",
	}

	// Handle Lookup Mode
	if *lookupCode != "" {
		if cfg.HSLKey == "" {
			log.Fatal("HSL API key is missing. Please configure it in config.json or secrets.txt")
		}
		// Directly construct an HSLFetcher for lookup so we can access LookupStop
		innerHSL := &fetcher.HSLFetcher{Config: cfg, Store: st}
		id, err := innerHSL.LookupStop(*lookupCode)
		if err != nil {
			log.Fatalf("Error looking up stop: %v", err)
		}
		fmt.Printf("Resolved code %s to GTFS stop id: %s\n", *lookupCode, id)
		return
	}

	fmiFetcher := &fetcher.LoggingFetcher{
		Fetcher: &fetcher.FMIFetcher{Config: cfg, Store: st},
		Store:   st,
		Name:    "FMI",
	}
	elecFetcher := &fetcher.LoggingFetcher{
		Fetcher: &fetcher.ElectricityFetcher{Config: cfg, Store: st},
		Store:   st,
		Name:    "Electricity",
	}

	// Start background jobs
	go runTicker(cfg.TransportInterval, hslFetcher)
	go runTicker(cfg.WeatherInterval, fmiFetcher)
	go runTicker(cfg.ElectricityInterval, elecFetcher)

	// Initial fetch
	go hslFetcher.Fetch()
	go fmiFetcher.Fetch()
	go elecFetcher.Fetch()

	// Device Stats Ticker
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			st.UpdateDeviceInfo(store.DeviceInfo{
				Uptime:       time.Since(startTime).String(),
				NumGoroutine: runtime.NumGoroutine(),
				MemAlloc:     fmt.Sprintf("%v MiB", m.Alloc/1024/1024),
				SysMem:       fmt.Sprintf("%v MiB", m.Sys/1024/1024),
				NumCPU:       runtime.NumCPU(),
			})
		}
	}()

	// HTTP Server
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := st.Get()
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("Error encoding response: %v", err)
		}
	})

	http.HandleFunc("/api/debug/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Create a masked config copy
		maskedCfg := *cfg
		if maskedCfg.HSLKey != "" {
			maskedCfg.HSLKey = "***MASKED***"
		}

		resp := struct {
			Config config.Config `json:"config"`
			Store  store.Data    `json:"store"`
		}{
			Config: maskedCfg,
			Store:  st.Get(),
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("Error encoding debug status: %v", err)
		}
	})

	http.HandleFunc("/api/debug/timeline", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := st.GetDebugData()
		if err := json.NewEncoder(w).Encode(data.APICalls); err != nil {
			log.Printf("Error encoding timeline: %v", err)
		}
	})

	http.HandleFunc("/api/debug/logs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := st.GetDebugData()
		if err := json.NewEncoder(w).Encode(data.AppLogs); err != nil {
			log.Printf("Error encoding logs: %v", err)
		}
	})

	http.HandleFunc("/api/debug/device", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := st.GetDebugData()
		if err := json.NewEncoder(w).Encode(data.Device); err != nil {
			log.Printf("Error encoding device info: %v", err)
		}
	})

	// Serve static files
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	log.Printf("Server starting on %s", cfg.Port)
	if err := http.ListenAndServe(cfg.Port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func runTicker(interval time.Duration, f fetcher.Fetcher) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := f.Fetch(); err != nil {
			log.Printf("Error fetching data: %v", err)
		}
	}
}

// LogWriter captures logs to store and stdout
type LogWriter struct {
	Target io.Writer
	Store  *store.Store
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	// Write to store (async to avoid blocking?) - keeping it sync for simplicity now
	// Copy buffer to avoid race conditions if p is reused
	msg := string(p)
	w.Store.AddLog(msg)
	return w.Target.Write(p)
}
