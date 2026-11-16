package httpapi

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// Taille maximale d'une grille de vent demandée par la carte : au-delà, la
// réponse grossit sans rien apporter à l'écran.
const windFieldMaxCells = 256

// windFieldBounds délimite la zone affichée, en degrés.
type windFieldBounds struct {
	South, North, West, East float64
}

// windFieldResponse donne la vitesse du vent (nœuds) aux nœuds d'une grille
// régulière en Mercator : les lignes vont du nord au sud, également espacées
// en ordonnée Mercator (comme les pixels de la carte), les colonnes d'ouest en
// est. Une valeur négative signale un point sans vent connu.
type windFieldResponse struct {
	Time   string    `json:"time"`
	South  float64   `json:"south"`
	North  float64   `json:"north"`
	West   float64   `json:"west"`
	East   float64   `json:"east"`
	Cols   int       `json:"cols"`
	Rows   int       `json:"rows"`
	Speeds []float64 `json:"speeds"`
}

// Les grilles du champ de vent ont leur propre cache : passer par celui des
// calculs en chasserait les échéances d'une route à chaque déplacement de la
// carte. Seules les échéances de la dernière demande sont gardées.
var windFieldCache = struct {
	sync.Mutex
	grids map[string]cachedGrid
}{grids: make(map[string]cachedGrid)}

// windFieldHandler répond à GET /api/weather/wind?time=&south=&north=&west=&east=&cols=&rows=.
func windFieldHandler(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	instant, err := time.Parse(time.RFC3339, query.Get("time"))
	if err != nil {
		writeJSONError(response, http.StatusBadRequest, "time doit être une date RFC 3339")
		return
	}
	var bounds windFieldBounds
	for _, field := range []struct {
		name  string
		value *float64
	}{{"south", &bounds.South}, {"north", &bounds.North}, {"west", &bounds.West}, {"east", &bounds.East}} {
		*field.value, err = strconv.ParseFloat(query.Get(field.name), 64)
		if err != nil || math.IsNaN(*field.value) || math.IsInf(*field.value, 0) {
			writeJSONError(response, http.StatusBadRequest, field.name+" doit être un nombre")
			return
		}
	}
	bounds.South = math.Max(bounds.South, -85)
	bounds.North = math.Min(bounds.North, 85)
	bounds.West = math.Max(bounds.West, -180)
	bounds.East = math.Min(bounds.East, 180)
	if bounds.South >= bounds.North || bounds.West >= bounds.East {
		writeJSONError(response, http.StatusBadRequest, "zone vide ou inversée")
		return
	}
	cols, colsErr := strconv.Atoi(query.Get("cols"))
	rows, rowsErr := strconv.Atoi(query.Get("rows"))
	if colsErr != nil || rowsErr != nil || cols < 2 || rows < 2 || cols > windFieldMaxCells || rows > windFieldMaxCells {
		writeJSONError(response, http.StatusBadRequest, fmt.Sprintf("cols et rows doivent être compris entre 2 et %d", windFieldMaxCells))
		return
	}

	status := gribWatcher.Status()
	files, _ := blendLatestGRIB(status.Active.Files, status)
	selected, used := windFieldFiles(files, instant)
	if len(selected) == 0 {
		writeJSONError(response, http.StatusServiceUnavailable, "aucune échéance GRIB disponible")
		return
	}
	provider, err := loadWindFieldProvider(selected)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, err.Error())
		return
	}
	speeds, err := sampleWindField(request.Context(), provider, used, bounds, cols, rows)
	if err != nil {
		writeJSONError(response, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(response, http.StatusOK, windFieldResponse{
		Time:  used.UTC().Format(time.RFC3339),
		South: bounds.South, North: bounds.North, West: bounds.West, East: bounds.East,
		Cols: cols, Rows: rows, Speeds: speeds,
	})
}

// windFieldFiles garde les deux échéances qui encadrent instant. Hors de la
// période couverte, l'instant est ramené à l'échéance la plus proche, qui
// suffit seule.
func windFieldFiles(files []string, instant time.Time) ([]string, time.Time) {
	type dated struct {
		path  string
		valid time.Time
	}
	ordered := make([]dated, 0, len(files))
	for _, path := range files {
		if _, valid, ok := gribFileValidity(path); ok {
			ordered = append(ordered, dated{path, valid})
		}
	}
	if len(ordered) == 0 {
		return nil, instant
	}
	sort.SliceStable(ordered, func(left, right int) bool { return ordered[left].valid.Before(ordered[right].valid) })
	first, last := ordered[0], ordered[len(ordered)-1]
	if !instant.After(first.valid) {
		return []string{first.path}, first.valid
	}
	if !instant.Before(last.valid) {
		return []string{last.path}, last.valid
	}
	upper := sort.Search(len(ordered), func(index int) bool { return !ordered[index].valid.Before(instant) })
	if ordered[upper].valid.Equal(instant) {
		return []string{ordered[upper].path}, instant
	}
	return []string{ordered[upper-1].path, ordered[upper].path}, instant
}

// loadWindFieldProvider charge les échéances demandées, en reprenant celles
// déjà en mémoire pour un calcul de route.
func loadWindFieldProvider(paths []string) (weather.WeatherProvider, error) {
	windFieldCache.Lock()
	defer windFieldCache.Unlock()
	requested := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		requested[filepath.Clean(path)] = struct{}{}
	}
	for key := range windFieldCache.grids {
		if _, keep := requested[key]; !keep {
			delete(windFieldCache.grids, key)
		}
	}
	grids := make([]weather.GRIB2WindGrid, 0, len(paths))
	for _, path := range paths {
		key, signature, err := cachedFileSignature(path)
		if err != nil {
			return nil, fmt.Errorf("weather: %w", err)
		}
		if cached, ok := windFieldCache.grids[key]; ok && cached.signature == signature {
			grids = append(grids, cached.grid)
			continue
		}
		routeDataCache.Lock()
		cached, ok := routeDataCache.grids[key]
		routeDataCache.Unlock()
		if !ok || cached.signature != signature {
			grid, err := weather.LoadGRIB2WindGrid(path)
			if err != nil {
				return nil, fmt.Errorf("weather: chargement de %q impossible: %w", path, err)
			}
			cached = cachedGrid{signature: signature, grid: grid}
		}
		windFieldCache.grids[key] = cached
		grids = append(grids, cached.grid)
	}
	return weather.NewGRIB2WeatherProvider(grids...)
}

func mercatorY(latitude float64) float64 {
	return math.Log(math.Tan(math.Pi/4 + latitude*math.Pi/360))
}

func latitudeFromMercatorY(y float64) float64 {
	return (2*math.Atan(math.Exp(y)) - math.Pi/2) * 180 / math.Pi
}

// sampleWindField calcule la vitesse du vent en nœuds, arrondie au dixième,
// aux nœuds de la grille décrite par windFieldResponse.
func sampleWindField(
	ctx context.Context,
	provider weather.WeatherProvider,
	instant time.Time,
	bounds windFieldBounds,
	cols, rows int,
) ([]float64, error) {
	speeds := make([]float64, 0, cols*rows)
	top, bottom := mercatorY(bounds.North), mercatorY(bounds.South)
	for row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		latitude := latitudeFromMercatorY(top + (bottom-top)*float64(row)/float64(rows-1))
		for col := range cols {
			longitude := bounds.West + (bounds.East-bounds.West)*float64(col)/float64(cols-1)
			wind, err := provider.WindAt(ctx, navigation.Position{
				Latitude:  navigation.Latitude(latitude),
				Longitude: navigation.Longitude(longitude),
			}, instant)
			if err != nil {
				speeds = append(speeds, -1)
				continue
			}
			knots, err := navigation.KnotsFromMetersPerSecond(wind.Speed)
			if err != nil {
				speeds = append(speeds, -1)
				continue
			}
			speeds = append(speeds, math.Round(float64(knots)*10)/10)
		}
	}
	return speeds, nil
}
