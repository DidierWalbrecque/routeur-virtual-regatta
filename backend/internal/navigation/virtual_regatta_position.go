package navigation

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	coordinatePairSeparator = regexp.MustCompile(`\s*[-–—]\s*`)
	coordinatePattern       = regexp.MustCompile(
		`(?i)^\s*([0-9]{1,3})\s*°\s*([NSEW])?\s*([0-9]{1,2})\s*['’′]\s*([0-9]+(?:\.[0-9]+)?)\s*["”″]\s*([NSEW])?\s*$`,
	)
)

// ParseVirtualRegattaPosition convertit une position en degrés, minutes et
// secondes en Position. L'hémisphère peut suivre les degrés (13°N 07'16") ou
// terminer la coordonnée (13°07'16.0"N), et les secondes peuvent être décimales.
func ParseVirtualRegattaPosition(value string) (Position, error) {
	parts := coordinatePairSeparator.Split(strings.TrimSpace(value), -1)
	if len(parts) != 2 {
		return Position{}, fmt.Errorf(
			"navigation: format attendu: 47°26'11.95\"N - 24°54'55.32\"W",
		)
	}

	latitude, err := parseDMSCoordinate(parts[0], true)
	if err != nil {
		return Position{}, fmt.Errorf(
			"navigation: latitude invalide: %w",
			err,
		)
	}
	longitude, err := parseDMSCoordinate(parts[1], false)
	if err != nil {
		return Position{}, fmt.Errorf(
			"navigation: longitude invalide: %w",
			err,
		)
	}

	position := Position{
		Latitude:  Latitude(latitude),
		Longitude: Longitude(longitude),
	}
	if err := position.Validate(); err != nil {
		return Position{}, err
	}
	return position, nil
}

// FormatVirtualRegattaPosition formate une position comme l'affichage de
// Virtual Regatta.
func FormatVirtualRegattaPosition(position Position) (string, error) {
	if err := position.Validate(); err != nil {
		return "", fmt.Errorf(
			"navigation: formatage de position impossible: %w",
			err,
		)
	}
	return fmt.Sprintf(
		"%s - %s",
		formatDMSCoordinate(float64(position.Latitude), "N", "S"),
		formatDMSCoordinate(float64(position.Longitude), "E", "W"),
	), nil
}

func parseDMSCoordinate(token string, isLatitude bool) (float64, error) {
	matches := coordinatePattern.FindStringSubmatch(
		strings.ToUpper(strings.TrimSpace(token)),
	)
	if len(matches) != 6 {
		return 0, fmt.Errorf("format DMS invalide: %q", token)
	}

	hemisphere := matches[2]
	if matches[5] != "" {
		if hemisphere != "" {
			return 0, fmt.Errorf("hémisphère en double dans %q", token)
		}
		hemisphere = matches[5]
	}
	if hemisphere == "" {
		return 0, fmt.Errorf("hémisphère manquant dans %q", token)
	}
	if isLatitude && hemisphere != "N" && hemisphere != "S" {
		return 0, fmt.Errorf(
			"hémisphère %q invalide pour une latitude",
			hemisphere,
		)
	}
	if !isLatitude && hemisphere != "E" && hemisphere != "W" {
		return 0, fmt.Errorf(
			"hémisphère %q invalide pour une longitude",
			hemisphere,
		)
	}

	degrees, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("degrés invalides: %w", err)
	}
	minutes, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, fmt.Errorf("minutes invalides: %w", err)
	}
	seconds, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return 0, fmt.Errorf("secondes invalides: %w", err)
	}
	if minutes >= 60 || seconds >= 60 {
		return 0, fmt.Errorf(
			"les minutes et secondes doivent être comprises entre 0 et 59",
		)
	}

	coordinate := float64(degrees) +
		float64(minutes)/60 +
		seconds/3600
	if hemisphere == "S" || hemisphere == "W" {
		coordinate = -coordinate
	}
	return coordinate, nil
}

func formatDMSCoordinate(
	coordinate float64,
	positiveHemisphere string,
	negativeHemisphere string,
) string {
	hemisphere := positiveHemisphere
	if coordinate < 0 {
		hemisphere = negativeHemisphere
	}

	totalSeconds := int(math.Round(math.Abs(coordinate) * 3600))
	degrees := totalSeconds / 3600
	minutes := totalSeconds % 3600 / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf(
		"%d°%s %02d'%02d\"",
		degrees,
		hemisphere,
		minutes,
		seconds,
	)
}
