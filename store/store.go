package store

import (
	"sync"
	"time"
)

// WeatherDataPoint holds a single point of weather info
type WeatherDataPoint struct {
	Temperature   float64   `json:"temperature"`
	WindSpeed     float64   `json:"wind_speed"`
	Precipitation float64   `json:"precipitation"`
	Pop           float64   `json:"pop"` // Probability of Precipitation
	Symbol        string    `json:"symbol"`
	Time          time.Time `json:"time"`
}

// WeatherData holds current and forecast
type WeatherData struct {
	Current  WeatherDataPoint   `json:"current"`
	Forecast []WeatherDataPoint `json:"forecast"`
}

// StopData holds info for a specific stop
type StopData struct {
	StopName   string      `json:"stop_name"`
	Departures []Departure `json:"departures"`
}

// TransportData holds list of stops
type TransportData struct {
	Stops     []StopData `json:"stops"`
	Timestamp time.Time  `json:"timestamp"`
}

type Departure struct {
	RouteNumber string    `json:"route_number"`
	Destination string    `json:"destination"`
	Time        time.Time `json:"time"`
	Realtime    bool      `json:"realtime"`
}

// ElectricityData holds current and future prices
type ElectricityData struct {
	CurrentPrice float64     `json:"current_price"` // c/kWh
	Prices       []PriceInfo `json:"prices"`        // Next 24h or so
	Timestamp    time.Time   `json:"timestamp"`
}

type PriceInfo struct {
	Price     float64   `json:"price"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Data is the aggregate state
type Data struct {
	Weather     WeatherData     `json:"weather"`
	Transport   TransportData   `json:"transport"`
	Electricity ElectricityData `json:"electricity"`
	APICalls    []APICallLog    `json:"-"` // Don't expose in main status
	AppLogs     []LogEntry      `json:"-"` // Don't expose in main status
	Device      DeviceInfo      `json:"-"` // Don't expose in main status
}

// Store is a thread-safe container for Data
type Store struct {
	mu   sync.RWMutex
	data Data
}

func New() *Store {
	return &Store{}
}

func (s *Store) Get() Data {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func (s *Store) UpdateWeather(w WeatherData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Weather = w
}

func (s *Store) UpdateTransport(t TransportData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Transport = t
}

func (s *Store) UpdateElectricity(e ElectricityData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Electricity = e
}

// --- Debug / Monitoring ---

type APICallLog struct {
	Timestamp time.Time `json:"timestamp"`
	Duration  string    `json:"duration"`
	URL       string    `json:"url"`
	Status    string    `json:"status"` // "success" or "error"
	Error     string    `json:"error,omitempty"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

type DeviceInfo struct {
	Uptime       string `json:"uptime"`
	NumGoroutine int    `json:"num_goroutine"`
	MemAlloc     string `json:"mem_alloc"`
	SysMem       string `json:"sys_mem"`
	NumCPU       int    `json:"num_cpu"`
}

type DebugData struct {
	APICalls []APICallLog `json:"api_calls"`
	AppLogs  []LogEntry   `json:"app_logs"`
	Device   DeviceInfo   `json:"device"`
}

func (s *Store) AddAPICallLog(log APICallLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Keep last 50 calls
	if len(s.data.APICalls) >= 50 {
		s.data.APICalls = s.data.APICalls[1:]
	}
	s.data.APICalls = append(s.data.APICalls, log)
}

func (s *Store) GetDebugData() DebugData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return DebugData{
		APICalls: append([]APICallLog(nil), s.data.APICalls...), // Copy
		AppLogs:  append([]LogEntry(nil), s.data.AppLogs...),    // Copy
		Device:   s.data.Device,
	}
}

func (s *Store) AddLog(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Keep last 100 logs
	if len(s.data.AppLogs) >= 100 {
		s.data.AppLogs = s.data.AppLogs[1:]
	}
	s.data.AppLogs = append(s.data.AppLogs, LogEntry{
		Timestamp: time.Now(),
		Message:   msg,
	})
}

func (s *Store) UpdateDeviceInfo(d DeviceInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Device = d
}
