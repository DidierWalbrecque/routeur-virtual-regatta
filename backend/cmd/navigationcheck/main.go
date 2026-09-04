package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("navigationcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)

	positionText := flags.String(
		"position",
		"",
		"position Virtual Regatta en degrés, minutes et secondes",
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
	heading := flags.Float64(
		"heading",
		math.NaN(),
		"cap constant en degrés depuis le nord",
	)
	speed := flags.Float64(
		"speed",
		math.NaN(),
		"vitesse moyenne constante en nœuds",
	)
	minutes := flags.Float64(
		"minutes",
		math.NaN(),
		"durée du déplacement en minutes",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf(
			"navigationcheck: arguments inattendus: %s",
			strings.Join(flags.Args(), " "),
		)
	}
	var from navigation.Position
	if strings.TrimSpace(*positionText) != "" {
		if !math.IsNaN(*latitude) || !math.IsNaN(*longitude) {
			return errors.New(
				"navigationcheck: -position ne peut pas être combiné avec -latitude ou -longitude",
			)
		}
		parsedPosition, err := parseVirtualRegattaPosition(*positionText)
		if err != nil {
			return fmt.Errorf(
				"navigationcheck: position Virtual Regatta invalide: %w",
				err,
			)
		}
		from = parsedPosition
	} else {
		if math.IsNaN(*latitude) {
			return errors.New(
				"navigationcheck: le paramètre -latitude ou -position est obligatoire",
			)
		}
		if math.IsNaN(*longitude) {
			return errors.New(
				"navigationcheck: le paramètre -longitude ou -position est obligatoire",
			)
		}
		from = navigation.Position{
			Latitude:  navigation.Latitude(*latitude),
			Longitude: navigation.Longitude(*longitude),
		}
	}
	if math.IsNaN(*heading) {
		return errors.New(
			"navigationcheck: le paramètre -heading est obligatoire",
		)
	}
	if math.IsNaN(*speed) {
		return errors.New(
			"navigationcheck: le paramètre -speed est obligatoire",
		)
	}
	if math.IsNaN(*minutes) {
		return errors.New(
			"navigationcheck: le paramètre -minutes est obligatoire",
		)
	}

	if err := from.Validate(); err != nil {
		return fmt.Errorf(
			"navigationcheck: position initiale invalide: %w",
			err,
		)
	}

	initialHeading := navigation.Heading(*heading)
	if err := initialHeading.Validate(); err != nil {
		return fmt.Errorf(
			"navigationcheck: cap invalide: %w",
			err,
		)
	}

	if math.IsInf(*speed, 0) || *speed < 0 {
		return errors.New(
			"navigationcheck: la vitesse doit être un nombre fini positif ou nul",
		)
	}
	if math.IsInf(*minutes, 0) || *minutes < 0 {
		return errors.New(
			"navigationcheck: la durée doit être un nombre fini positif ou nul",
		)
	}

	distance := navigation.NauticalMiles(*speed * *minutes / 60)
	to, err := navigation.DestinationPosition(
		from,
		initialHeading,
		distance,
	)
	if err != nil {
		return fmt.Errorf(
			"navigationcheck: calcul du déplacement impossible: %w",
			err,
		)
	}

	fmt.Fprintf(
		stdout,
		"Position initiale VR : %s\n",
		formatVirtualRegattaPosition(from),
	)
	fmt.Fprintf(
		stdout,
		"Position initiale : %.6f°, %.6f°\n",
		from.Latitude,
		from.Longitude,
	)
	fmt.Fprintf(stdout, "Cap constant : %.2f°\n", initialHeading)
	fmt.Fprintf(stdout, "Vitesse constante : %.6f nd\n", *speed)
	fmt.Fprintf(stdout, "Durée : %.2f min\n", *minutes)
	fmt.Fprintf(stdout, "Distance parcourue : %.6f NM\n", distance)
	fmt.Fprintf(
		stdout,
		"Position théorique VR : %s\n",
		formatVirtualRegattaPosition(to),
	)
	fmt.Fprintf(
		stdout,
		"Position théorique : %.6f°, %.6f°\n",
		to.Latitude,
		to.Longitude,
	)

	return nil
}
