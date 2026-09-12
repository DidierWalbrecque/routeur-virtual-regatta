// Command routecheck computes a first multi-layer isochrone route from local
// GRIB2 forecasts, a boat polar and manually entered race positions.
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
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/weather"
)

const maximumLayerCount = 10_000

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
	files          filePaths
	polarPath      string
	position       navigation.Position
	destination    navigation.Position
	instant        time.Time
	totalDuration  time.Duration
	stepDuration   time.Duration
	layerCount     int
	owned          equipment.EquipmentSet
	headingSpacing float64
	cellSize       navigation.NauticalMiles
	maxCandidates  int
	arrivalRadius  navigation.NauticalMiles
	stamina        simulation.StaminaState
	geoJSONPath    string
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
			"routecheck: chargement de la météo impossible: %w",
			err,
		)
	}
	boatPolar, err := polar.LoadFile(options.polarPath)
	if err != nil {
		return fmt.Errorf(
			"routecheck: chargement de la polaire impossible: %w",
			err,
		)
	}

	search, err := routing.BuildIsochroneSearch(
		context.Background(),
		weatherProvider,
		boatPolar,
		routing.IsochroneSearchInput{
			Position:       options.position,
			Destination:    options.destination,
			Instant:        options.instant,
			StepDuration:   options.stepDuration,
			LayerCount:     options.layerCount,
			Owned:          options.owned,
			HeadingSpacing: options.headingSpacing,
			CellSize:       options.cellSize,
			MaxCandidates:  options.maxCandidates,
			ArrivalRadius:  options.arrivalRadius,
			InitialStamina: options.stamina,
		},
	)
	if err != nil {
		return fmt.Errorf("routecheck: calcul de la route impossible: %w", err)
	}

	route, err := search.ReconstructBestRoute()
	if err != nil {
		return fmt.Errorf(
			"routecheck: reconstruction de la route impossible: %w",
			err,
		)
	}
	if err := writeRouteGeoJSON(options.geoJSONPath, route); err != nil {
		return err
	}
	return writeResult(stdout, boatPolar, options, search, route)
}

func writeResult(
	stdout io.Writer,
	boatPolar *polar.Polar,
	options options,
	search routing.IsochroneSearch,
	route routing.IsochroneRoute,
) error {
	initialPosition, err := navigation.FormatVirtualRegattaPosition(
		options.position,
	)
	if err != nil {
		return fmt.Errorf(
			"routecheck: formatage de la position initiale impossible: %w",
			err,
		)
	}
	destination, err := navigation.FormatVirtualRegattaPosition(
		options.destination,
	)
	if err != nil {
		return fmt.Errorf(
			"routecheck: formatage de la destination impossible: %w",
			err,
		)
	}

	fmt.Fprintf(stdout, "Bateau : %s\n", boatPolar.Label)
	fmt.Fprintf(stdout, "Départ VR : %s\n", initialPosition)
	fmt.Fprintf(stdout, "Destination VR : %s\n", destination)
	fmt.Fprintf(stdout, "Instant initial : %s\n", options.instant.Format(time.RFC3339))
	fmt.Fprintf(
		stdout,
		"Horizon : %.2f min | pas %.2f min | %d fronts\n",
		options.totalDuration.Minutes(),
		options.stepDuration.Minutes(),
		options.layerCount,
	)
	fmt.Fprintf(
		stdout,
		"Exploration : caps tous les %.2f° | cellules min. %.3f NM | limite %d | arrivée %.3f NM\n",
		options.headingSpacing,
		options.cellSize,
		options.maxCandidates,
		options.arrivalRadius,
	)
	fmt.Fprintln(stdout, "Route retenue :")

	for index, routeStep := range route.Steps {
		step := routeStep.Node.Candidate.Step
		position, err := navigation.FormatVirtualRegattaPosition(
			step.NextPosition,
		)
		if err != nil {
			return fmt.Errorf(
				"routecheck: formatage de la position du pas %d impossible: %w",
				index+1,
				err,
			)
		}
		fmt.Fprintf(
			stdout,
			"%d. %s | cap %.2f° | %s\n",
			index+1,
			step.NextInstant.Format(time.RFC3339),
			step.Heading,
			position,
		)
		fmt.Fprintf(
			stdout,
			"   vent %.3f nd à %.2f° | TWA %.2f° %s | voile %s | vitesse %.3f nd\n",
			step.WindKnots,
			step.Wind.Direction,
			step.WindAngle,
			formatTack(step.Tack),
			step.BestSail.SailName,
			step.BestSail.Speed,
		)
		fmt.Fprintf(
			stdout,
			"   distance restante %.3f NM | progression globale %+.3f NM\n",
			routeStep.Node.RemainingDistance,
			routeStep.Node.Progress,
		)
	}

	final := route.Steps[len(route.Steps)-1]
	finalPosition, err := navigation.FormatVirtualRegattaPosition(
		final.Node.Candidate.Step.NextPosition,
	)
	if err != nil {
		return fmt.Errorf(
			"routecheck: formatage de la position finale impossible: %w",
			err,
		)
	}
	fmt.Fprintf(stdout, "Arrivée de l'horizon : %s\n", route.FinishedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "Position finale VR : %s\n", finalPosition)
	fmt.Fprintf(
		stdout,
		"Distance restante : %.3f NM\n",
		final.Node.RemainingDistance,
	)
	fmt.Fprintf(
		stdout,
		"Progression globale : %+.3f NM\n",
		final.Node.Progress,
	)
	reached := "non"
	if search.Reached {
		reached = "oui"
	}
	fmt.Fprintf(stdout, "Destination atteinte : %s\n", reached)
	fmt.Fprint(stdout, "Positions conservées par front :")
	for _, frontier := range search.Frontiers {
		fmt.Fprintf(stdout, " %d", len(frontier.Nodes))
	}
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, "Cellules effectives par front :")
	for _, frontier := range search.Frontiers {
		fmt.Fprintf(stdout, " %.3f", frontier.CellSize)
	}
	fmt.Fprintln(stdout)
	if options.geoJSONPath != "" {
		fmt.Fprintf(stdout, "GeoJSON : %s\n", options.geoJSONPath)
	}

	return nil
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	result := options{}
	flags := flag.NewFlagSet("routecheck", flag.ContinueOnError)
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
		"position initiale au format Virtual Regatta",
	)
	latitude := flags.Float64(
		"latitude",
		math.NaN(),
		"latitude initiale en degrés décimaux",
	)
	longitude := flags.Float64(
		"longitude",
		math.NaN(),
		"longitude initiale en degrés décimaux",
	)
	destinationText := flags.String(
		"destination",
		"",
		"destination au format Virtual Regatta",
	)
	destinationLatitude := flags.Float64(
		"destination-latitude",
		math.NaN(),
		"latitude de destination en degrés décimaux",
	)
	destinationLongitude := flags.Float64(
		"destination-longitude",
		math.NaN(),
		"longitude de destination en degrés décimaux",
	)
	atText := flags.String(
		"at",
		"",
		"instant initial au format RFC3339",
	)
	minutes := flags.Float64(
		"minutes",
		math.NaN(),
		"horizon total de calcul en minutes",
	)
	stepMinutes := flags.Float64(
		"step-minutes",
		10,
		"durée d'un front d'isochrone en minutes",
	)
	headingSpacing := flags.Float64(
		"heading-spacing",
		10,
		"écart entre les caps explorés en degrés",
	)
	cellSize := flags.Float64(
		"cell-size",
		1,
		"taille minimale d'une cellule d'élagage en milles nautiques",
	)
	maxCandidates := flags.Int(
		"max-candidates",
		500,
		"nombre maximal de positions conservées par front ; 0 désactive la limite",
	)
	arrivalRadius := flags.Float64(
		"arrival-radius",
		1,
		"distance maximale de la destination considérée comme une arrivée en milles nautiques",
	)
	stamina := flags.Float64(
		"stamina",
		100,
		"stamina initiale du bateau entre 0 et 100",
	)
	geoJSONPath := flags.String(
		"geojson",
		"",
		"chemin optionnel du fichier GeoJSON à produire",
	)
	ownedText := flags.String(
		"owned",
		"",
		"équipements possédés séparés par des virgules",
	)

	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf(
			"routecheck: arguments invalides: %w",
			err,
		)
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf(
			"routecheck: arguments inattendus: %s",
			strings.Join(flags.Args(), " "),
		)
	}
	if len(result.files) == 0 {
		return options{}, errors.New(
			"routecheck: au moins un paramètre -file est obligatoire",
		)
	}
	if strings.TrimSpace(*polarPath) == "" {
		return options{}, errors.New(
			"routecheck: le paramètre -polar est obligatoire",
		)
	}

	position, err := parsePosition(
		"position initiale",
		*positionText,
		*latitude,
		*longitude,
	)
	if err != nil {
		return options{}, err
	}
	destination, err := parsePosition(
		"destination",
		*destinationText,
		*destinationLatitude,
		*destinationLongitude,
	)
	if err != nil {
		return options{}, err
	}

	if strings.TrimSpace(*atText) == "" {
		return options{}, errors.New(
			"routecheck: le paramètre -at est obligatoire",
		)
	}
	instant, err := time.Parse(time.RFC3339, *atText)
	if err != nil {
		return options{}, fmt.Errorf(
			"routecheck: instant %q invalide, format RFC3339 attendu: %w",
			*atText,
			err,
		)
	}

	if math.IsNaN(*minutes) ||
		math.IsInf(*minutes, 0) ||
		*minutes <= 0 {
		return options{}, errors.New(
			"routecheck: -minutes doit être strictement positif",
		)
	}
	if math.IsNaN(*stepMinutes) ||
		math.IsInf(*stepMinutes, 0) ||
		*stepMinutes <= 0 {
		return options{}, errors.New(
			"routecheck: -step-minutes doit être strictement positif",
		)
	}
	layerCountValue := *minutes / *stepMinutes
	layerCount := int(math.Round(layerCountValue))
	if math.Abs(layerCountValue-float64(layerCount)) > 1e-9 {
		return options{}, errors.New(
			"routecheck: -minutes doit être un multiple exact de -step-minutes",
		)
	}
	if layerCount <= 0 || layerCount > maximumLayerCount {
		return options{}, fmt.Errorf(
			"routecheck: le nombre de fronts doit être compris entre 1 et %d",
			maximumLayerCount,
		)
	}

	if _, err := routing.GenerateHeadingGrid(*headingSpacing); err != nil {
		return options{}, fmt.Errorf(
			"routecheck: -heading-spacing invalide: %w",
			err,
		)
	}
	cellSizeValue := navigation.NauticalMiles(*cellSize)
	if math.IsNaN(*cellSize) ||
		math.IsInf(*cellSize, 0) ||
		cellSizeValue <= 0 {
		return options{}, errors.New(
			"routecheck: -cell-size doit être strictement positif",
		)
	}
	if *maxCandidates != 0 && *maxCandidates < 4 {
		return options{}, errors.New(
			"routecheck: -max-candidates doit être nul ou au moins égal à 4",
		)
	}
	arrivalRadiusValue := navigation.NauticalMiles(*arrivalRadius)
	if math.IsNaN(*arrivalRadius) ||
		math.IsInf(*arrivalRadius, 0) ||
		arrivalRadiusValue < 0 {
		return options{}, errors.New(
			"routecheck: -arrival-radius doit être positif ou nul",
		)
	}

	staminaState := simulation.StaminaState{
		Level: simulation.StaminaLevel(*stamina),
	}
	if err := staminaState.Validate(); err != nil {
		return options{}, fmt.Errorf(
			"routecheck: -stamina invalide: %w",
			err,
		)
	}

	owned, err := equipment.ParseSet(*ownedText)
	if err != nil {
		return options{}, fmt.Errorf(
			"routecheck: équipements possédés invalides: %w",
			err,
		)
	}

	result.polarPath = *polarPath
	result.position = position
	result.destination = destination
	result.instant = instant
	result.totalDuration = time.Duration(*minutes * float64(time.Minute))
	result.stepDuration = time.Duration(*stepMinutes * float64(time.Minute))
	result.layerCount = layerCount
	result.owned = owned
	result.headingSpacing = *headingSpacing
	result.cellSize = cellSizeValue
	result.maxCandidates = *maxCandidates
	result.arrivalRadius = arrivalRadiusValue
	result.stamina = staminaState
	result.geoJSONPath = strings.TrimSpace(*geoJSONPath)
	return result, nil
}

func parsePosition(
	name,
	positionText string,
	latitude,
	longitude float64,
) (navigation.Position, error) {
	if strings.TrimSpace(positionText) != "" {
		if !math.IsNaN(latitude) || !math.IsNaN(longitude) {
			return navigation.Position{}, fmt.Errorf(
				"routecheck: %s: le format VR ne peut pas être combiné avec les coordonnées décimales",
				name,
			)
		}
		position, err := navigation.ParseVirtualRegattaPosition(
			positionText,
		)
		if err != nil {
			return navigation.Position{}, fmt.Errorf(
				"routecheck: %s Virtual Regatta invalide: %w",
				name,
				err,
			)
		}
		return position, nil
	}

	if math.IsNaN(latitude) || math.IsNaN(longitude) {
		return navigation.Position{}, fmt.Errorf(
			"routecheck: fournir %s au format VR ou ses deux coordonnées décimales",
			name,
		)
	}
	position := navigation.Position{
		Latitude:  navigation.Latitude(latitude),
		Longitude: navigation.Longitude(longitude),
	}
	if err := position.Validate(); err != nil {
		return navigation.Position{}, fmt.Errorf(
			"routecheck: %s invalide: %w",
			name,
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
