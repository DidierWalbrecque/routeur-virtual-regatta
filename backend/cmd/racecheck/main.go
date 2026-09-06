// Command racecheck simulates a constant-heading course with real weather,
// a boat polar and the owned equipment.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

type filePaths []string

func (paths *filePaths) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("le chemin GRIB2 ne doit pas être vide")
	}
	*paths = append(*paths, value)
	return nil
}

func (paths *filePaths) String() string {
	return strings.Join(*paths, ",")
}

type options struct {
	files       filePaths
	polarPath   string
	position    navigation.Position
	instant     time.Time
	heading     navigation.Heading
	minutes     float64
	stepMinutes float64
	owned       equipment.EquipmentSet
}

type raceStepInput struct {
	Position navigation.Position
	Instant  time.Time
	Heading  navigation.Heading
	Minutes  float64
	Owned    equipment.EquipmentSet
}

type raceStepResult struct {
	Wind         navigation.Wind
	WindKnots    navigation.Knots
	WindAngle    navigation.TrueWindAngle
	Tack         navigation.Tack
	BestSail     polar.SailPerformance
	Distance     navigation.NauticalMiles
	NextPosition navigation.Position
	NextInstant  time.Time
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	options, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}

	weatherProvider, err := weather.LoadGRIB2WeatherProvider(
		[]string(options.files)...,
	)
	if err != nil {
		return fmt.Errorf(
			"racecheck: chargement de la météo impossible: %w",
			err,
		)
	}
	boatPolar, err := polar.LoadFile(options.polarPath)
	if err != nil {
		return fmt.Errorf(
			"racecheck: chargement de la polaire impossible: %w",
			err,
		)
	}

	result, err := simulateRace(
		context.Background(),
		weatherProvider,
		boatPolar,
		raceSimulationInput{
			Position:    options.position,
			Instant:     options.instant,
			Heading:     options.heading,
			Minutes:     options.minutes,
			StepMinutes: options.stepMinutes,
			Owned:       options.owned,
		},
	)
	if err != nil {
		return err
	}
	lastStep := result.Steps[len(result.Steps)-1]

	initialPosition, err := navigation.FormatVirtualRegattaPosition(
		options.position,
	)
	if err != nil {
		return fmt.Errorf(
			"racecheck: formatage de la position initiale impossible: %w",
			err,
		)
	}
	nextPosition, err := navigation.FormatVirtualRegattaPosition(
		result.NextPosition,
	)
	if err != nil {
		return fmt.Errorf(
			"racecheck: formatage de la position suivante impossible: %w",
			err,
		)
	}

	fmt.Fprintf(stdout, "Bateau : %s\n", boatPolar.Label)
	fmt.Fprintf(stdout, "Position initiale VR : %s\n", initialPosition)
	fmt.Fprintf(
		stdout,
		"Position initiale : %.6f°, %.6f°\n",
		options.position.Latitude,
		options.position.Longitude,
	)
	fmt.Fprintf(stdout, "Instant initial : %s\n", options.instant.Format(time.RFC3339))
	fmt.Fprintf(stdout, "Cap constant : %.2f°\n", options.heading)
	fmt.Fprintf(
		stdout,
		"Pas de calcul : %.2f min | %d pas\n",
		options.stepMinutes,
		len(result.Steps),
	)
	fmt.Fprintf(
		stdout,
		"Vent NOAA au dernier pas : %.6f nd | direction %.3f°\n",
		lastStep.WindKnots,
		lastStep.Wind.Direction,
	)
	fmt.Fprintf(
		stdout,
		"TWA au dernier pas : %.3f° | amure %s\n",
		lastStep.WindAngle,
		formatTack(lastStep.Tack),
	)
	fmt.Fprintf(stdout, "Voile au dernier pas : %s\n", lastStep.BestSail.SailName)
	fmt.Fprintf(
		stdout,
		"Vitesse au dernier pas : %.6f nd\n",
		lastStep.BestSail.Speed,
	)
	fmt.Fprintf(stdout, "Durée simulée : %.2f min\n", options.minutes)
	fmt.Fprintf(stdout, "Distance parcourue : %.6f NM\n", result.Distance)
	fmt.Fprintf(stdout, "Instant suivant : %s\n", result.NextInstant.Format(time.RFC3339))
	fmt.Fprintf(stdout, "Position suivante VR : %s\n", nextPosition)
	fmt.Fprintf(
		stdout,
		"Position suivante : %.6f°, %.6f°\n",
		result.NextPosition.Latitude,
		result.NextPosition.Longitude,
	)

	return nil
}

func evaluateRaceStep(
	ctx context.Context,
	weatherProvider weather.WeatherProvider,
	boatPolar *polar.Polar,
	input raceStepInput,
) (raceStepResult, error) {
	if weatherProvider == nil {
		return raceStepResult{}, errors.New(
			"racecheck: fournisseur météo requis",
		)
	}
	if boatPolar == nil {
		return raceStepResult{}, errors.New("racecheck: polaire requise")
	}
	if err := input.Position.Validate(); err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: position invalide: %w",
			err,
		)
	}
	if input.Instant.IsZero() {
		return raceStepResult{}, errors.New(
			"racecheck: instant initial requis",
		)
	}
	if err := input.Heading.Validate(); err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: cap invalide: %w",
			err,
		)
	}
	if math.IsNaN(input.Minutes) ||
		math.IsInf(input.Minutes, 0) ||
		input.Minutes <= 0 {
		return raceStepResult{}, errors.New(
			"racecheck: la durée doit être strictement positive",
		)
	}
	if err := input.Owned.Validate(); err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: équipements invalides: %w",
			err,
		)
	}

	wind, err := weatherProvider.WindAt(
		ctx,
		input.Position,
		input.Instant,
	)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: calcul du vent impossible: %w",
			err,
		)
	}
	windKnots, err := navigation.KnotsFromMetersPerSecond(wind.Speed)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: conversion du vent impossible: %w",
			err,
		)
	}
	windAngle, err := navigation.TrueWindAngleFromHeading(
		input.Heading,
		wind.Direction,
	)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: calcul du TWA impossible: %w",
			err,
		)
	}
	tack, err := navigation.TackFromHeading(
		input.Heading,
		wind.Direction,
	)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: calcul de l'amure impossible: %w",
			err,
		)
	}

	effects, err := equipment.ResolveEffects(boatPolar, input.Owned)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: résolution des équipements impossible: %w",
			err,
		)
	}
	bestSail, err := boatPolar.BestEffectiveSailAt(
		polar.WindSpeed(windKnots),
		polar.TrueWindAngle(windAngle),
		effects.AvailableSails,
		effects.SpeedOptions,
	)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: calcul de la vitesse impossible: %w",
			err,
		)
	}

	distance, err := navigation.DistanceAtConstantSpeed(
		navigation.Knots(bestSail.Speed),
		navigation.Seconds(input.Minutes*60),
	)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: calcul de la distance impossible: %w",
			err,
		)
	}
	nextPosition, err := navigation.DestinationPosition(
		input.Position,
		input.Heading,
		distance,
	)
	if err != nil {
		return raceStepResult{}, fmt.Errorf(
			"racecheck: calcul de la position suivante impossible: %w",
			err,
		)
	}

	duration := time.Duration(input.Minutes * float64(time.Minute))
	return raceStepResult{
		Wind:         wind,
		WindKnots:    windKnots,
		WindAngle:    windAngle,
		Tack:         tack,
		BestSail:     bestSail,
		Distance:     distance,
		NextPosition: nextPosition,
		NextInstant:  input.Instant.Add(duration),
	}, nil
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	result := options{}
	flags := flag.NewFlagSet("racecheck", flag.ContinueOnError)
	flags.SetOutput(stderr)

	flags.Var(
		&result.files,
		"file",
		"chemin d'un fichier GRIB2 ; option répétable",
	)
	polarPath := flags.String(
		"polar",
		"",
		"chemin du fichier JSON de polaire",
	)
	positionText := flags.String(
		"position",
		"",
		"position au format Virtual Regatta",
	)
	latitude := flags.Float64(
		"latitude",
		math.NaN(),
		"latitude en degrés décimaux",
	)
	longitude := flags.Float64(
		"longitude",
		math.NaN(),
		"longitude en degrés décimaux",
	)
	atText := flags.String(
		"at",
		"",
		"instant initial au format RFC3339",
	)
	heading := flags.Float64(
		"heading",
		math.NaN(),
		"cap constant en degrés",
	)
	minutes := flags.Float64(
		"minutes",
		math.NaN(),
		"durée totale à simuler en minutes",
	)
	stepMinutes := flags.Float64(
		"step-minutes",
		10,
		"durée maximale d'un pas de calcul en minutes",
	)
	ownedText := flags.String(
		"owned",
		"",
		"équipements possédés séparés par des virgules",
	)

	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf(
			"racecheck: arguments invalides: %w",
			err,
		)
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf(
			"racecheck: arguments inattendus: %s",
			strings.Join(flags.Args(), " "),
		)
	}
	if len(result.files) == 0 {
		return options{}, errors.New(
			"racecheck: au moins un paramètre -file est obligatoire",
		)
	}
	if strings.TrimSpace(*polarPath) == "" {
		return options{}, errors.New(
			"racecheck: le paramètre -polar est obligatoire",
		)
	}

	position, err := parsePosition(
		*positionText,
		*latitude,
		*longitude,
	)
	if err != nil {
		return options{}, err
	}
	if strings.TrimSpace(*atText) == "" {
		return options{}, errors.New(
			"racecheck: le paramètre -at est obligatoire",
		)
	}
	instant, err := time.Parse(time.RFC3339, *atText)
	if err != nil {
		return options{}, fmt.Errorf(
			"racecheck: instant %q invalide, format RFC3339 attendu: %w",
			*atText,
			err,
		)
	}
	if math.IsNaN(*heading) {
		return options{}, errors.New(
			"racecheck: le paramètre -heading est obligatoire",
		)
	}
	initialHeading := navigation.Heading(*heading)
	if err := initialHeading.Validate(); err != nil {
		return options{}, fmt.Errorf(
			"racecheck: cap invalide: %w",
			err,
		)
	}
	if math.IsNaN(*minutes) ||
		math.IsInf(*minutes, 0) ||
		*minutes <= 0 {
		return options{}, errors.New(
			"racecheck: -minutes doit être strictement positif",
		)
	}
	if math.IsNaN(*stepMinutes) ||
		math.IsInf(*stepMinutes, 0) ||
		*stepMinutes <= 0 {
		return options{}, errors.New(
			"racecheck: -step-minutes doit être strictement positif",
		)
	}
	owned, err := equipment.ParseSet(*ownedText)
	if err != nil {
		return options{}, fmt.Errorf(
			"racecheck: équipements possédés invalides: %w",
			err,
		)
	}

	result.polarPath = *polarPath
	result.position = position
	result.instant = instant
	result.heading = initialHeading
	result.minutes = *minutes
	result.stepMinutes = *stepMinutes
	result.owned = owned
	return result, nil
}

func parsePosition(
	positionText string,
	latitude,
	longitude float64,
) (navigation.Position, error) {
	if strings.TrimSpace(positionText) != "" {
		if !math.IsNaN(latitude) || !math.IsNaN(longitude) {
			return navigation.Position{}, errors.New(
				"racecheck: -position ne peut pas être combiné avec -latitude ou -longitude",
			)
		}
		position, err := navigation.ParseVirtualRegattaPosition(
			positionText,
		)
		if err != nil {
			return navigation.Position{}, fmt.Errorf(
				"racecheck: position Virtual Regatta invalide: %w",
				err,
			)
		}
		return position, nil
	}

	if math.IsNaN(latitude) || math.IsNaN(longitude) {
		return navigation.Position{}, errors.New(
			"racecheck: fournir -position ou -latitude avec -longitude",
		)
	}
	position := navigation.Position{
		Latitude:  navigation.Latitude(latitude),
		Longitude: navigation.Longitude(longitude),
	}
	if err := position.Validate(); err != nil {
		return navigation.Position{}, fmt.Errorf(
			"racecheck: position invalide: %w",
			err,
		)
	}
	return position, nil
}

func formatTack(tack navigation.Tack) string {
	switch tack {
	case navigation.TackPort:
		return "bâbord"
	case navigation.TackStarboard:
		return "tribord"
	default:
		return "indéfinie"
	}
}
