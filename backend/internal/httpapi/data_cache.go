package httpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/land"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type fileSignature struct {
	size     int64
	modified int64
}

type cachedGrid struct {
	signature fileSignature
	grid      weather.GRIB2WindGrid
}

type cachedPolar struct {
	signature fileSignature
	polar     *polar.Polar
}

type cachedLand struct {
	signature  fileSignature
	constraint *land.GeoJSONConstraint
}

type cacheLoadStats struct {
	Hits   int
	Misses int
}

var routeDataCache = struct {
	sync.Mutex
	grids  map[string]cachedGrid
	polars map[string]cachedPolar
	lands  map[string]cachedLand
}{
	grids:  make(map[string]cachedGrid),
	polars: make(map[string]cachedPolar),
	lands:  make(map[string]cachedLand),
}

func cachedFileSignature(path string) (string, fileSignature, error) {
	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return cleanPath, fileSignature{}, fmt.Errorf("inspection de %q impossible: %w", path, err)
	}
	return cleanPath, fileSignature{size: info.Size(), modified: info.ModTime().UnixNano()}, nil
}

func loadCachedWeatherProvider(paths ...string) (*weather.GRIB2WeatherProvider, cacheLoadStats, error) {
	if len(paths) == 0 {
		return nil, cacheLoadStats{}, fmt.Errorf("weather: au moins un fichier GRIB2 est requis")
	}

	routeDataCache.Lock()
	defer routeDataCache.Unlock()

	grids := make([]weather.GRIB2WindGrid, 0, len(paths))
	stats := cacheLoadStats{}
	for _, path := range paths {
		key, signature, err := cachedFileSignature(path)
		if err != nil {
			return nil, stats, fmt.Errorf("weather: %w", err)
		}
		cached, exists := routeDataCache.grids[key]
		if !exists || cached.signature != signature {
			stats.Misses++
			grid, err := weather.LoadGRIB2WindGrid(path)
			if err != nil {
				return nil, stats, fmt.Errorf("weather: chargement de %q impossible: %w", path, err)
			}
			cached = cachedGrid{signature: signature, grid: grid}
			routeDataCache.grids[key] = cached
		} else {
			stats.Hits++
		}
		grids = append(grids, cached.grid)
	}
	provider, err := weather.NewGRIB2WeatherProvider(grids...)
	return provider, stats, err
}

func loadCachedPolar(path string) (*polar.Polar, bool, error) {
	routeDataCache.Lock()
	defer routeDataCache.Unlock()

	key, signature, err := cachedFileSignature(path)
	if err != nil {
		return nil, false, fmt.Errorf("polar: %w", err)
	}
	if cached, exists := routeDataCache.polars[key]; exists && cached.signature == signature {
		return cached.polar, true, nil
	}
	value, err := polar.LoadFile(path)
	if err != nil {
		return nil, false, err
	}
	routeDataCache.polars[key] = cachedPolar{signature: signature, polar: value}
	return value, false, nil
}

func loadCachedLand(path string) (*land.GeoJSONConstraint, bool, error) {
	routeDataCache.Lock()
	defer routeDataCache.Unlock()

	key, signature, err := cachedFileSignature(path)
	if err != nil {
		return nil, false, fmt.Errorf("land: %w", err)
	}
	if cached, exists := routeDataCache.lands[key]; exists && cached.signature == signature {
		return cached.constraint, true, nil
	}
	value, err := land.LoadGeoJSONFile(path)
	if err != nil {
		return nil, false, err
	}
	routeDataCache.lands[key] = cachedLand{signature: signature, constraint: value}
	return value, false, nil
}
