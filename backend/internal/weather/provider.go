// Package weather définit l'accès aux prévisions météorologiques utilisées par
// le moteur de routage.
package weather

import (
	"context"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// Forecast représente un vent prévu à une position et une échéance données.
type Forecast struct {
	Position navigation.Position
	ValidAt  time.Time
	Wind     navigation.Wind
}

// Validate vérifie la position, l'échéance et le vent de la prévision.
func (forecast Forecast) Validate() error {
	if err := forecast.Position.Validate(); err != nil {
		return fmt.Errorf("weather: position invalide: %w", err)
	}
	if forecast.ValidAt.IsZero() {
		return fmt.Errorf(
			"weather: l'échéance de la prévision ne doit pas être nulle",
		)
	}
	if err := forecast.Wind.Validate(); err != nil {
		return fmt.Errorf("weather: vent invalide: %w", err)
	}
	return nil
}

// WeatherProvider fournit le vent prévu pour une position et un instant.
//
// L'implémentation GRIB2 pourra effectuer lecture, interpolation et mise en
// cache sans exposer ces détails au moteur de routage.
type WeatherProvider interface {
	WindAt(
		ctx context.Context,
		position navigation.Position,
		instant time.Time,
	) (navigation.Wind, error)
}
