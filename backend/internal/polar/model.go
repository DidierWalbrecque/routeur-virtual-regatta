// Package polar contient les structures et les fonctions nécessaires pour
// charger les polaires de vitesse utilisées par Virtual Regatta Online.
package polar

import "time"

// BoatID identifie un modèle de bateau dans le dépôt de polaires.
type BoatID int

// SailID identifie une voile dans une polaire.
type SailID int

// WindSpeed représente une vitesse de vent réel, exprimée en nœuds.
type WindSpeed float64

// BoatSpeed représente une vitesse de bateau, exprimée en nœuds.
type BoatSpeed float64

// TrueWindAngle représente un angle au vent réel, exprimé en degrés.
type TrueWindAngle float64

// SpeedRatio représente un coefficient multiplicateur sans unité.
type SpeedRatio float64

// Polar contient les données validées d'un modèle de bateau.
type Polar struct {
	BoatID                  BoatID
	Label                   string
	GlobalSpeedRatio        SpeedRatio
	IceSpeedRatio           SpeedRatio
	AutoSailChangeTolerance SpeedRatio
	BadSailTolerance        SpeedRatio
	MaxSpeed                BoatSpeed
	UpdatedAt               time.Time

	WindSpeeds []WindSpeed
	WindAngles []TrueWindAngle
	Sails      []Sail

	Foil  FoilProfile
	Hull  HullProfile
	Winch WinchProfile
}

// Sail contient l'identité d'une voile et sa grille de vitesses.
type Sail struct {
	ID     SailID
	Name   string
	Speeds SpeedGrid
}

// SpeedGrid stocke les vitesses avec la convention Values[twaIndex][twsIndex].
type SpeedGrid struct {
	Values [][]BoatSpeed
}

// FoilProfile décrit le bonus des foils et leur zone d'activation.
type FoilProfile struct {
	MaxSpeedRatio SpeedRatio
	TWAMin        TrueWindAngle
	TWAMax        TrueWindAngle
	TWAMerge      TrueWindAngle
	TWSMin        WindSpeed
	TWSMax        WindSpeed
	TWSMerge      WindSpeed
}

// HullProfile décrit le coefficient apporté par le polish de coque.
type HullProfile struct {
	SpeedRatio SpeedRatio
}

// ManeuverPenalty décrit le ralentissement et la durée d'une manœuvre.
type ManeuverPenalty struct {
	SpeedRatio SpeedRatio
	Duration   time.Duration
}

// WindDependentPenalty contient les pénalités de vent faible et de vent fort.
type WindDependentPenalty struct {
	LowWind  ManeuverPenalty
	HighWind ManeuverPenalty
}

// ManeuverProfile conserve les règles standard, professionnelles et anciennes.
// Standard et Professional valent nil dans les anciens fichiers de polaires.
type ManeuverProfile struct {
	Standard     *WindDependentPenalty
	Professional *WindDependentPenalty
	LegacyStd    ManeuverPenalty
	LegacyPro    ManeuverPenalty
}

// WinchProfile regroupe les pénalités des trois types de manœuvre.
// Les seuils peuvent être nuls dans les anciens fichiers qui ne les déclarent pas.
type WinchProfile struct {
	LowWindThreshold  WindSpeed
	HighWindThreshold WindSpeed
	Tack              ManeuverProfile
	Gybe              ManeuverProfile
	SailChange        ManeuverProfile
}
