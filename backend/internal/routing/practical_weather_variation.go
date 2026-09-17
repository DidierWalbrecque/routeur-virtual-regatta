package routing

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

// PracticalWeatherVariation describes a perturbation applied to every weather
// lookup of a practical-heading scenario. SpeedOffset is expressed in metres
// per second and DirectionOffset in degrees.
type PracticalWeatherVariation struct {
	SpeedOffset     float64
	DirectionOffset float64
}

// Validate checks that a weather variation can be applied to a forecast.
func (variation PracticalWeatherVariation) Validate() error {
	for name, value := range map[string]float64{
		"écart de vitesse":   variation.SpeedOffset,
		"écart de direction": variation.DirectionOffset,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf(
				"routing: variation météo pratique: %s fini requis",
				name,
			)
		}
	}
	return nil
}

type practicalWeatherVariationProvider struct {
	provider  weather.WeatherProvider
	variation PracticalWeatherVariation
}

func (provider practicalWeatherVariationProvider) WindAt(
	ctx context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	wind, err := provider.provider.WindAt(ctx, position, instant)
	if err != nil {
		return navigation.Wind{}, err
	}
	if err := wind.Validate(); err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"routing: variation météo pratique: vent source invalide: %w",
			err,
		)
	}

	direction := math.Mod(
		float64(wind.Direction)+provider.variation.DirectionOffset,
		360,
	)
	if direction < 0 {
		direction += 360
	}
	adjusted := navigation.Wind{
		Speed: navigation.MetersPerSecond(math.Max(
			0,
			float64(wind.Speed)+provider.variation.SpeedOffset,
		)),
		Direction: navigation.WindDirection(direction),
	}
	if err := adjusted.Validate(); err != nil {
		return navigation.Wind{}, fmt.Errorf(
			"routing: variation météo pratique invalide: %w",
			err,
		)
	}
	return adjusted, nil
}
