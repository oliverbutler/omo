package mapping

import (
	"errors"
	"sync/atomic"
	"time"
)

type MappingService struct {
	tripCache      atomic.Value
	tripCacheReady chan struct{}
	isLoaded       atomic.Bool
}

func NewMappingService() *MappingService {
	ms := &MappingService{
		tripCacheReady: make(chan struct{}),
	}
	go ms.loadTripCache()
	return ms
}

func (ms *MappingService) loadTripCache() {
	// Simulating cache loading
	time.Sleep(2 * time.Second)

	trips, err := ReadTripData()
	if err != nil {
		// Handle error (log it, set a flag, etc.)
		close(ms.tripCacheReady)
		return
	}

	ms.tripCache.Store(trips)
	ms.isLoaded.Store(true)
	close(ms.tripCacheReady)
}

func (ms *MappingService) GetTrips() ([]Trip, error) {
	if !ms.isLoaded.Load() {
		select {
		case <-ms.tripCacheReady:
			// Cache is ready, proceed
		case <-time.After(5 * time.Second):
			// Timeout if cache takes too long to load
			return nil, errors.New("cache is still loading, please try again later")
		}
	}

	if !ms.isLoaded.Load() {
		return nil, errors.New("trip data failed to load")
	}

	return ms.tripCache.Load().([]Trip), nil
}
