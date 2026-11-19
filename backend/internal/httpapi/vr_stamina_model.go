package httpapi

import (
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

// boatStaminaCoefficients maps a boat's polar id to its stamina-consumption
// coefficient, as published by the ITYC dashboard (boat2StaminaCoeff). Boats
// not listed here use the neutral coefficient of 1.
var boatStaminaCoefficients = map[polar.BoatID]float64{
	4:  1.2, // Imoca
	6:  1.5, // Ultim (Solo)
	7:  1.2, // Volvo 65
	9:  1.5, // Ultim (Crew)
	10: 1.5, // Olympus
	13: 2,   // Caravelle
	14: 1.5, // Super Maxi 100
	16: 2,   // Tara
	19: 1.2, // Mod70
	20: 1.2, // Cruiser Racer
	21: 1.5, // Ultim BP XI
}

// boatStaminaCoefficient returns the stamina-consumption coefficient of a boat.
func boatStaminaCoefficient(id polar.BoatID) float64 {
	if coefficient, ok := boatStaminaCoefficients[id]; ok {
		return coefficient
	}
	return 1
}

// staminaRecoveryRules returns the Virtual Regatta stamina-recovery model: the
// time to recover one stamina point rises with a cosine profile from 5 minutes
// at 0 kn to 15 minutes at 30 kn of true wind.
func staminaRecoveryRules() simulation.StaminaRecoveryRules {
	return simulation.StaminaRecoveryRules{
		Points:   1,
		LowWind:  0,
		HighWind: 30,
		LowTime:  5 * time.Minute,
		HighTime: 15 * time.Minute,
	}
}
