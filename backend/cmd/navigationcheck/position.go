package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

var virtualRegattaPositionPattern = regexp.MustCompile(
	`^\s*([0-9]{1,2})°\s*([NS])\s+([0-9]{1,2})['′]\s*([0-9]{1,2})["″]\s*[-–—]\s*([0-9]{1,3})°\s*([EW])\s+([0-9]{1,2})['′]\s*([0-9]{1,2})["″]\s*$`,
)

func parseVirtualRegattaPosition(value string) (navigation.Position, error) {
	matches := virtualRegattaPositionPattern.FindStringSubmatch(
		strings.ToUpper(strings.TrimSpace(value)),
	)
	if len(matches) != 9 {
		return navigation.Position{}, fmt.Errorf(
			"format attendu: 13°N 04'08\" - 121°E 37'42\"",
		)
	}

	latitude, err := parseDMSCoordinate(
		matches[1],
		matches[2],
		matches[3],
		matches[4],
	)
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"latitude invalide: %w",
			err,
		)
	}
	longitude, err := parseDMSCoordinate(
		matches[5],
		matches[6],
		matches[7],
		matches[8],
	)
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"longitude invalide: %w",
			err,
		)
	}

	position := navigation.Position{
		Latitude:  navigation.Latitude(latitude),
		Longitude: navigation.Longitude(longitude),
	}
	if err := position.Validate(); err != nil {
		return navigation.Position{}, err
	}
	return position, nil
}

func parseDMSCoordinate(
	degreesText string,
	hemisphere string,
	minutesText string,
	secondsText string,
) (float64, error) {
	degrees, err := strconv.Atoi(degreesText)
	if err != nil {
		return 0, fmt.Errorf("degrés invalides: %w", err)
	}
	minutes, err := strconv.Atoi(minutesText)
	if err != nil {
		return 0, fmt.Errorf("minutes invalides: %w", err)
	}
	seconds, err := strconv.Atoi(secondsText)
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
		float64(seconds)/3600
	if hemisphere == "S" || hemisphere == "W" {
		coordinate = -coordinate
	}
	return coordinate, nil
}
