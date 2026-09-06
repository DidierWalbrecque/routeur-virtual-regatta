package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/equipment"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	flags := flag.NewFlagSet("polarcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)

	polarPath := flags.String(
		"polar",
		"",
		"chemin du fichier JSON de polaire",
	)
	windSpeed := flags.Float64(
		"tws",
		math.NaN(),
		"vitesse du vent réel en nœuds",
	)
	windAngle := flags.Float64(
		"twa",
		math.NaN(),
		"angle au vent réel en degrés",
	)
	category := flags.Int(
		"category",
		6,
		"catégorie de course comprise entre 1 et 6",
	)
	budget := flags.Int(
		"budget",
		-1,
		"plafond total de crédits, bonus inclus",
	)
	owned := flags.String(
		"owned",
		"",
		"équipements possédés séparés par des virgules",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("polarcheck: arguments inattendus: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*polarPath) == "" {
		return errors.New("polarcheck: le paramètre -polar est obligatoire")
	}
	if math.IsNaN(*windSpeed) {
		return errors.New("polarcheck: le paramètre -tws est obligatoire")
	}
	if math.IsNaN(*windAngle) {
		return errors.New("polarcheck: le paramètre -twa est obligatoire")
	}
	if *budget < 0 {
		return errors.New("polarcheck: le paramètre -budget est obligatoire et doit être positif ou nul")
	}

	ownedSet, err := parseEquipmentSet(*owned)
	if err != nil {
		return fmt.Errorf("polarcheck: lecture des équipements possédés impossible: %w", err)
	}

	configuration := equipment.RaceConfiguration{
		Budget: equipment.RaceBudget{
			Category:   equipment.RaceCategory(*category),
			TotalLimit: equipment.Credits(*budget),
		},
		Owned: ownedSet,
	}
	if err := configuration.Validate(); err != nil {
		return fmt.Errorf("polarcheck: configuration de course invalide: %w", err)
	}

	boatPolar, err := polar.LoadFile(*polarPath)
	if err != nil {
		return fmt.Errorf("polarcheck: chargement de la polaire impossible: %w", err)
	}

	effects, err := equipment.ResolveEffects(boatPolar, ownedSet)
	if err != nil {
		return fmt.Errorf("polarcheck: résolution des équipements impossible: %w", err)
	}

	current, err := boatPolar.BestEffectiveSailAt(
		polar.WindSpeed(*windSpeed),
		polar.TrueWindAngle(*windAngle),
		effects.AvailableSails,
		effects.SpeedOptions,
	)
	if err != nil {
		return fmt.Errorf("polarcheck: calcul de la vitesse actuelle impossible: %w", err)
	}

	currentCost, err := equipment.BreakdownAt(configuration.Budget.Category, ownedSet)
	if err != nil {
		return fmt.Errorf("polarcheck: calcul du coût actuel impossible: %w", err)
	}

	purchases, err := equipment.RankNextPurchasesAt(
		boatPolar,
		configuration,
		polar.WindSpeed(*windSpeed),
		polar.TrueWindAngle(*windAngle),
	)
	if err != nil {
		return fmt.Errorf("polarcheck: classement des achats impossible: %w", err)
	}

	fmt.Fprintf(stdout, "Bateau : %s\n", boatPolar.Label)
	fmt.Fprintf(stdout, "Vent : TWS %.2f nd | TWA %.2f°\n", *windSpeed, *windAngle)
	fmt.Fprintf(stdout, "Catégorie : %d | plafond total : %d crédits\n", *category, *budget)
	fmt.Fprintf(stdout, "Équipements possédés : %s\n", formatEquipmentSet(ownedSet))
	fmt.Fprintf(
		stdout,
		"Coût actuel : %d crédits (%d crédits personnels)\n",
		currentCost.TotalCost,
		currentCost.PersonalCredits,
	)
	fmt.Fprintf(stdout, "Meilleure voile : %s\n", current.SailName)
	fmt.Fprintf(stdout, "Vitesse attendue : %.6f nd\n", current.Speed)

	if len(purchases) == 0 {
		fmt.Fprintln(stdout, "Aucun nouvel équipement n'est abordable.")
		return nil
	}

	fmt.Fprintln(stdout, "Prochains achats abordables :")
	for index, purchase := range purchases {
		fmt.Fprintf(
			stdout,
			"%d. %s | achat %d | personnel +%d | voile %s | vitesse %.6f nd | gain %+.6f nd\n",
			index+1,
			purchase.Equipment.String(),
			purchase.PurchaseCost,
			purchase.AdditionalPersonalCredits,
			purchase.BestSail.SailName,
			purchase.BestSail.Speed,
			purchase.SpeedGain,
		)
	}

	return nil
}

func parseEquipmentSet(value string) (equipment.EquipmentSet, error) {
	return equipment.ParseSet(value)
}

func formatEquipmentSet(set equipment.EquipmentSet) string {
	names := make([]string, 0, len(equipment.All()))
	for _, item := range equipment.All() {
		if set.Has(item) {
			names = append(names, item.String())
		}
	}
	if len(names) == 0 {
		return "aucun"
	}
	return strings.Join(names, ", ")
}
