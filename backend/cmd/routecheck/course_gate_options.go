package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

type courseGateTexts []string

func (values *courseGateTexts) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("la description de porte ne doit pas être vide")
	}
	*values = append(*values, value)
	return nil
}

func (values *courseGateTexts) String() string {
	return strings.Join(*values, ",")
}

func parseCourseGates(values courseGateTexts) ([]routing.CourseGate, error) {
	gates := make([]routing.CourseGate, 0, len(values))
	for index, value := range values {
		parts := strings.Split(value, "|")
		if len(parts) != 4 {
			return nil, fmt.Errorf(
				"routecheck: -gate %d invalide: format nom|marque 1|marque 2|sens attendu",
				index+1,
			)
		}

		firstMark, err := parseCourseGatePosition(parts[1])
		if err != nil {
			return nil, fmt.Errorf(
				"routecheck: -gate %d: première marque invalide: %w",
				index+1,
				err,
			)
		}
		secondMark, err := parseCourseGatePosition(parts[2])
		if err != nil {
			return nil, fmt.Errorf(
				"routecheck: -gate %d: seconde marque invalide: %w",
				index+1,
				err,
			)
		}
		direction, err := parseCourseGateDirection(parts[3])
		if err != nil {
			return nil, fmt.Errorf(
				"routecheck: -gate %d: %w",
				index+1,
				err,
			)
		}

		gate := routing.CourseGate{
			Name:       strings.TrimSpace(parts[0]),
			FirstMark:  firstMark,
			SecondMark: secondMark,
			Direction:  direction,
		}
		if err := gate.Validate(); err != nil {
			return nil, fmt.Errorf(
				"routecheck: -gate %d invalide: %w",
				index+1,
				err,
			)
		}
		gates = append(gates, gate)
	}
	return gates, nil
}

func parseCourseGatePosition(value string) (navigation.Position, error) {
	value = strings.TrimSpace(value)
	decimalParts := strings.Split(value, ",")
	if len(decimalParts) == 2 {
		latitude, err := strconv.ParseFloat(
			strings.TrimSpace(decimalParts[0]),
			64,
		)
		if err != nil {
			return navigation.Position{}, fmt.Errorf(
				"latitude décimale invalide: %w",
				err,
			)
		}
		longitude, err := strconv.ParseFloat(
			strings.TrimSpace(decimalParts[1]),
			64,
		)
		if err != nil {
			return navigation.Position{}, fmt.Errorf(
				"longitude décimale invalide: %w",
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

	position, err := navigation.ParseVirtualRegattaPosition(value)
	if err != nil {
		return navigation.Position{}, fmt.Errorf(
			"position Virtual Regatta invalide: %w",
			err,
		)
	}
	return position, nil
}

func parseCourseGateDirection(value string) (routing.GateDirection, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "either":
		return routing.GateEitherDirection, nil
	case "right-to-left":
		return routing.GateRightToLeft, nil
	case "left-to-right":
		return routing.GateLeftToRight, nil
	default:
		return routing.GateEitherDirection, fmt.Errorf(
			"sens %q invalide, valeurs attendues: either, right-to-left ou left-to-right",
			strings.TrimSpace(value),
		)
	}
}

func formatCourseGateDirection(direction routing.GateDirection) string {
	switch direction {
	case routing.GateEitherDirection:
		return "either"
	case routing.GateRightToLeft:
		return "right-to-left"
	case routing.GateLeftToRight:
		return "left-to-right"
	default:
		return fmt.Sprintf("unknown-%d", direction)
	}
}
