package fetcher

import (
	"rasp_info/store"
	"time"
)

// LoggingFetcher wraps a Fetcher and logs its execution
type LoggingFetcher struct {
	Fetcher Fetcher
	Store   *store.Store
	Name    string
}

func (l *LoggingFetcher) Fetch() error {
	start := time.Now()
	err := l.Fetcher.Fetch()
	duration := time.Since(start)

	status := "success"
	errorMsg := ""
	if err != nil {
		status = "error"
		errorMsg = err.Error()
	}

	// Log to store
	l.Store.AddAPICallLog(store.APICallLog{
		Timestamp: start,
		Duration:  duration.String(),
		URL:       l.Name, // Using Name as proxy for URL/Service
		Status:    status,
		Error:     errorMsg,
	})

	return err
}
