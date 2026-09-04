package weather

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

type fixedWeatherProvider struct {
	wind navigation.Wind
	err  error
}

func (provider fixedWeatherProvider) WindAt(
	ctx context.Context,
	position navigation.Position,
	instant time.Time,
) (navigation.Wind, error) {
	if err := ctx.Err(); err != nil {
		return navigation.Wind{}, err
	}
	return provider.wind, provider.err
}

func TestForecastValidate(t *testing.T) {
	forecast := Forecast{
		Position: navigation.Position{
			Latitude:  13.121111111111,
			Longitude: 121.586666666667,
		},
		ValidAt: time.Date(2026, time.September, 4, 18, 50, 0, 0, time.UTC),
		Wind: navigation.Wind{
			Speed:     2.263555555556,
			Direction: 222,
		},
	}

	if err := forecast.Validate(); err != nil {
		t.Fatalf("Forecast.Validate() a retourné une erreur: %v", err)
	}
}

func TestForecastValidateRejectsInvalidValues(t *testing.T) {
	validTime := time.Date(
		2026,
		time.September,
		4,
		18,
		50,
		0,
		0,
		time.UTC,
	)
	validPosition := navigation.Position{
		Latitude:  13,
		Longitude: 121,
	}
	validWind := navigation.Wind{
		Speed:     5,
		Direction: 90,
	}

	tests := []struct {
		name      string
		forecast  Forecast
		errorPart string
	}{
		{
			name: "position invalide",
			forecast: Forecast{
				Position: navigation.Position{Latitude: 91},
				ValidAt:  validTime,
				Wind:     validWind,
			},
			errorPart: "position",
		},
		{
			name: "échéance nulle",
			forecast: Forecast{
				Position: validPosition,
				Wind:     validWind,
			},
			errorPart: "échéance",
		},
		{
			name: "vent invalide",
			forecast: Forecast{
				Position: validPosition,
				ValidAt:  validTime,
				Wind: navigation.Wind{
					Speed:     -1,
					Direction: 0,
				},
			},
			errorPart: "vent",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.forecast.Validate()
			if err == nil {
				t.Fatal("la prévision invalide doit être refusée")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf(
					"erreur = %q, doit contenir %q",
					err,
					test.errorPart,
				)
			}
		})
	}
}

func TestWeatherProviderContract(t *testing.T) {
	want := navigation.Wind{
		Speed:     7.5,
		Direction: 315,
	}
	var provider WeatherProvider = fixedWeatherProvider{wind: want}

	got, err := provider.WindAt(
		context.Background(),
		navigation.Position{Latitude: 13, Longitude: 121},
		time.Date(2026, time.September, 4, 19, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("WeatherProvider.WindAt() a retourné une erreur: %v", err)
	}
	if got != want {
		t.Errorf("vent = %+v, attendu %+v", got, want)
	}
}

func TestWeatherProviderReceivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var provider WeatherProvider = fixedWeatherProvider{}
	_, err := provider.WindAt(
		ctx,
		navigation.Position{},
		time.Now(),
	)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("erreur = %v, attendue %v", err, context.Canceled)
	}
}
