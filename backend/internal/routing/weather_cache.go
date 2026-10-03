package routing

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type weatherCacheKey struct {
	position navigation.Position
	instant  time.Time
}

type weatherCacheEntry struct {
	ready chan struct{}
	wind  navigation.Wind
	err   error
}

// concurrentWeatherCache shares exact weather lookups between practical
// headings and their scenarios. An in-flight entry prevents identical
// requests from being evaluated more than once when headings run in parallel.
type concurrentWeatherCache struct {
	provider weather.WeatherProvider

	mu      sync.Mutex
	entries map[weatherCacheKey]*weatherCacheEntry
}

func newConcurrentWeatherCache(
	provider weather.WeatherProvider,
) *concurrentWeatherCache {
	return &concurrentWeatherCache{
		provider: provider,
		entries:  make(map[weatherCacheKey]*weatherCacheEntry),
	}
}

func (cache *concurrentWeatherCache) WindAt(
	ctx context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	if cache == nil || cache.provider == nil {
		return navigation.Wind{}, errors.New("routing: fournisseur météo requis")
	}
	key := weatherCacheKey{position: position, instant: instant}

	cache.mu.Lock()
	if entry, exists := cache.entries[key]; exists {
		cache.mu.Unlock()
		if ctx == nil {
			<-entry.ready
		} else {
			select {
			case <-entry.ready:
			case <-ctx.Done():
				return navigation.Wind{}, ctx.Err()
			}
		}
		return entry.wind, entry.err
	}

	entry := &weatherCacheEntry{ready: make(chan struct{})}
	cache.entries[key] = entry
	cache.mu.Unlock()

	wind, err := cache.provider.WindAt(ctx, position, instant)
	cache.mu.Lock()
	entry.wind = wind
	entry.err = err
	close(entry.ready)
	cache.mu.Unlock()
	return wind, err
}
