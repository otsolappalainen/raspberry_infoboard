package fetcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"rasp_info/config"
	"rasp_info/store"
	"time"
)

type ElectricityFetcher struct {
	Config *config.Config
	Store  *store.Store
}

type SpotPrice struct {
	PriceNoTax   float64 `json:"PriceNoTax"`
	PriceWithTax float64 `json:"PriceWithTax"`
	DateTime     string  `json:"DateTime"`
}

func (f *ElectricityFetcher) Fetch() error {
	resp, err := http.Get(f.Config.SpotAPIUrl)
	if err != nil {
		return fmt.Errorf("failed to fetch electricity prices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("electricity api returned status: %d", resp.StatusCode)
	}

	// Read body for logging
	var bodyBytes bytes.Buffer
	_, err = bodyBytes.ReadFrom(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read electricity body: %w", err)
	}
	// Log the raw response
	// log.Printf("Electricity Response: %s", bodyBytes.String())

	var prices []SpotPrice
	if err := json.Unmarshal(bodyBytes.Bytes(), &prices); err != nil {
		return fmt.Errorf("failed to decode electricity json: %w", err)
	}

	now := time.Now()
	windowEnd := now.Add(24 * time.Hour)
	var currentPrice float64
	var priceList []store.PriceInfo
	currentSet := false

	for _, p := range prices {
		t, err := time.Parse(time.RFC3339, p.DateTime)
		if err != nil {
			continue
		}

		slotEnd := t.Add(15 * time.Minute)
		if slotEnd.Before(now) {
			continue
		}
		if t.After(windowEnd) {
			break
		}

		priceValue := p.PriceWithTax * 100
		if !currentSet && now.After(t) && now.Before(slotEnd) {
			currentPrice = priceValue
			currentSet = true
		}

		priceList = append(priceList, store.PriceInfo{
			Price:     priceValue,
			StartTime: t,
			EndTime:   slotEnd,
		})

		if len(priceList) >= 96 {
			break
		}
	}

	if !currentSet && len(priceList) > 0 {
		currentPrice = priceList[0].Price
	}

	f.Store.UpdateElectricity(store.ElectricityData{
		CurrentPrice: currentPrice,
		Prices:       priceList,
		Timestamp:    now,
	})

	return nil
}
