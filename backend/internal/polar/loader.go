package polar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
)

// Les types suffixés par File reproduisent le format JSON externe. Ils restent
// privés afin que le reste du moteur dépende uniquement du modèle métier Polar.
type polarFile struct {
	ID                      int        `json:"_id"`
	Label                   string     `json:"label"`
	GlobalSpeedRatio        float64    `json:"globalSpeedRatio"`
	IceSpeedRatio           float64    `json:"iceSpeedRatio"`
	AutoSailChangeTolerance float64    `json:"autoSailChangeTolerance"`
	BadSailTolerance        float64    `json:"badSailTolerance"`
	MaxSpeed                float64    `json:"maxSpeed"`
	UpdatedAt               time.Time  `json:"_updatedAt"`
	Foil                    foilFile   `json:"foil"`
	Hull                    hullFile   `json:"hull"`
	Winch                   winchFile  `json:"winch"`
	TWS                     []float64  `json:"tws"`
	TWA                     []float64  `json:"twa"`
	Sails                   []sailFile `json:"sail"`
}

type sailFile struct {
	ID    int         `json:"id"`
	Name  string      `json:"name"`
	Speed [][]float64 `json:"speed"`
}

type foilFile struct {
	SpeedRatio float64 `json:"speedRatio"`
	TWAMin     float64 `json:"twaMin"`
	TWAMax     float64 `json:"twaMax"`
	TWAMerge   float64 `json:"twaMerge"`
	TWSMin     float64 `json:"twsMin"`
	TWSMax     float64 `json:"twsMax"`
	TWSMerge   float64 `json:"twsMerge"`
}

type hullFile struct {
	SpeedRatio float64 `json:"speedRatio"`
}

type winchFile struct {
	Tack       maneuverFile `json:"tack"`
	Gybe       maneuverFile `json:"gybe"`
	SailChange maneuverFile `json:"sailChange"`
	LWS        float64      `json:"lws"`
	HWS        float64      `json:"hws"`
}

type maneuverFile struct {
	StdTimerSec float64          `json:"stdTimerSec"`
	StdRatio    float64          `json:"stdRatio"`
	ProTimerSec float64          `json:"proTimerSec"`
	ProRatio    float64          `json:"proRatio"`
	Standard    *windPenaltyFile `json:"std"`
	Pro         *windPenaltyFile `json:"pro"`
}

type windPenaltyFile struct {
	LowWind  penaltyFile `json:"lw"`
	HighWind penaltyFile `json:"hw"`
}

type penaltyFile struct {
	Ratio float64 `json:"ratio"`
	Timer float64 `json:"timer"`
}

// Load décode et valide une polaire depuis un flux JSON.
func Load(reader io.Reader) (*Polar, error) {
	if reader == nil {
		return nil, errors.New("polar: le lecteur JSON est nil")
	}

	decoder := json.NewDecoder(reader)
	var source polarFile
	if err := decoder.Decode(&source); err != nil {
		return nil, fmt.Errorf("polar: décodage JSON impossible: %w", err)
	}

	// Un second décodage doit atteindre la fin du flux. Cette vérification évite
	// d'accepter silencieusement deux objets JSON concaténés dans le même fichier.
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("polar: données supplémentaires après l'objet JSON")
		}
		return nil, fmt.Errorf("polar: données supplémentaires invalides: %w", err)
	}

	result := source.toPolar()
	if err := result.Validate(); err != nil {
		return nil, err
	}

	return result, nil
}

// LoadFile ouvre, charge et valide une polaire depuis un fichier local.
func LoadFile(path string) (*Polar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("polar: ouverture de %q impossible: %w", path, err)
	}

	result, loadErr := Load(file)
	closeErr := file.Close()
	if loadErr != nil {
		return nil, fmt.Errorf("polar: chargement de %q impossible: %w", path, loadErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("polar: fermeture de %q impossible: %w", path, closeErr)
	}
	return result, nil
}

// Validate vérifie les invariants nécessaires aux futures interpolations.
func (polar *Polar) Validate() error {
	if polar == nil {
		return errors.New("polar: la polaire est nil")
	}
	if strings.TrimSpace(polar.Label) == "" {
		return errors.New("polar: le label est vide")
	}
	if err := validateWindSpeeds(polar.WindSpeeds); err != nil {
		return err
	}
	if err := validateWindAngles(polar.WindAngles); err != nil {
		return err
	}
	if err := validateFiniteNonNegative("maxSpeed", float64(polar.MaxSpeed)); err != nil {
		return err
	}
	if len(polar.Sails) == 0 {
		return errors.New("polar: aucune voile déclarée")
	}

	seenIDs := make(map[SailID]struct{}, len(polar.Sails))
	seenNames := make(map[string]struct{}, len(polar.Sails))
	for sailIndex, sail := range polar.Sails {
		if _, exists := seenIDs[sail.ID]; exists {
			return fmt.Errorf("polar: identifiant de voile dupliqué: %d", sail.ID)
		}
		seenIDs[sail.ID] = struct{}{}

		name := strings.TrimSpace(sail.Name)
		if name == "" {
			return fmt.Errorf("polar: nom vide pour la voile à l'index %d", sailIndex)
		}
		if _, exists := seenNames[name]; exists {
			return fmt.Errorf("polar: nom de voile dupliqué: %q", name)
		}
		seenNames[name] = struct{}{}

		if err := validateSpeedGrid(sail, len(polar.WindAngles), len(polar.WindSpeeds)); err != nil {
			return err
		}
	}

	return nil
}

func validateWindSpeeds(values []WindSpeed) error {
	if len(values) < 2 {
		return errors.New("polar: l'axe TWS doit contenir au moins deux valeurs")
	}
	for index, value := range values {
		if err := validateFiniteNonNegative(fmt.Sprintf("tws[%d]", index), float64(value)); err != nil {
			return err
		}
		if index > 0 && value <= values[index-1] {
			return fmt.Errorf("polar: l'axe TWS n'est pas strictement croissant à l'index %d", index)
		}
	}
	return nil
}

func validateWindAngles(values []TrueWindAngle) error {
	if len(values) < 2 {
		return errors.New("polar: l'axe TWA doit contenir au moins deux valeurs")
	}
	for index, value := range values {
		if !isFinite(float64(value)) || value < 0 || value > 180 {
			return fmt.Errorf("polar: twa[%d] doit être un angle fini compris entre 0 et 180 degrés", index)
		}
		if index > 0 && value <= values[index-1] {
			return fmt.Errorf("polar: l'axe TWA n'est pas strictement croissant à l'index %d", index)
		}
	}
	return nil
}

func validateSpeedGrid(sail Sail, expectedRows int, expectedColumns int) error {
	if len(sail.Speeds.Values) != expectedRows {
		return fmt.Errorf("polar: la voile %q possède %d lignes, attendu %d", sail.Name, len(sail.Speeds.Values), expectedRows)
	}
	for rowIndex, row := range sail.Speeds.Values {
		if len(row) != expectedColumns {
			return fmt.Errorf("polar: la voile %q, ligne %d, possède %d colonnes, attendu %d", sail.Name, rowIndex, len(row), expectedColumns)
		}
		for columnIndex, speed := range row {
			if err := validateFiniteNonNegative(fmt.Sprintf("sail[%s].speed[%d][%d]", sail.Name, rowIndex, columnIndex), float64(speed)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFiniteNonNegative(name string, value float64) error {
	if !isFinite(value) || value < 0 {
		return fmt.Errorf("polar: %s doit être un nombre fini positif ou nul", name)
	}
	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (source polarFile) toPolar() *Polar {
	result := &Polar{
		BoatID:                  BoatID(source.ID),
		Label:                   source.Label,
		GlobalSpeedRatio:        SpeedRatio(source.GlobalSpeedRatio),
		IceSpeedRatio:           SpeedRatio(source.IceSpeedRatio),
		AutoSailChangeTolerance: SpeedRatio(source.AutoSailChangeTolerance),
		BadSailTolerance:        SpeedRatio(source.BadSailTolerance),
		MaxSpeed:                BoatSpeed(source.MaxSpeed),
		UpdatedAt:               source.UpdatedAt,
		WindSpeeds:              convertWindSpeeds(source.TWS),
		WindAngles:              convertWindAngles(source.TWA),
		Sails:                   convertSails(source.Sails),
		Foil: FoilProfile{
			MaxSpeedRatio: SpeedRatio(source.Foil.SpeedRatio),
			TWAMin:        TrueWindAngle(source.Foil.TWAMin),
			TWAMax:        TrueWindAngle(source.Foil.TWAMax),
			TWAMerge:      TrueWindAngle(source.Foil.TWAMerge),
			TWSMin:        WindSpeed(source.Foil.TWSMin),
			TWSMax:        WindSpeed(source.Foil.TWSMax),
			TWSMerge:      WindSpeed(source.Foil.TWSMerge),
		},
		Hull: HullProfile{SpeedRatio: SpeedRatio(source.Hull.SpeedRatio)},
		Winch: WinchProfile{
			LowWindThreshold:  WindSpeed(source.Winch.LWS),
			HighWindThreshold: WindSpeed(source.Winch.HWS),
			Tack:              convertManeuver(source.Winch.Tack),
			Gybe:              convertManeuver(source.Winch.Gybe),
			SailChange:        convertManeuver(source.Winch.SailChange),
		},
	}
	return result
}

func convertWindSpeeds(values []float64) []WindSpeed {
	result := make([]WindSpeed, len(values))
	for index, value := range values {
		result[index] = WindSpeed(value)
	}
	return result
}

func convertWindAngles(values []float64) []TrueWindAngle {
	result := make([]TrueWindAngle, len(values))
	for index, value := range values {
		result[index] = TrueWindAngle(value)
	}
	return result
}

func convertSails(sources []sailFile) []Sail {
	result := make([]Sail, len(sources))
	for sailIndex, source := range sources {
		values := make([][]BoatSpeed, len(source.Speed))
		for rowIndex, row := range source.Speed {
			values[rowIndex] = make([]BoatSpeed, len(row))
			for columnIndex, speed := range row {
				values[rowIndex][columnIndex] = BoatSpeed(speed)
			}
		}
		result[sailIndex] = Sail{
			ID:     SailID(source.ID),
			Name:   source.Name,
			Speeds: SpeedGrid{Values: values},
		}
	}
	return result
}

func convertManeuver(source maneuverFile) ManeuverProfile {
	return ManeuverProfile{
		Standard:     convertWindPenalty(source.Standard),
		Professional: convertWindPenalty(source.Pro),
		LegacyStd: ManeuverPenalty{
			SpeedRatio: SpeedRatio(source.StdRatio),
			Duration:   secondsToDuration(source.StdTimerSec),
		},
		LegacyPro: ManeuverPenalty{
			SpeedRatio: SpeedRatio(source.ProRatio),
			Duration:   secondsToDuration(source.ProTimerSec),
		},
	}
}

func convertWindPenalty(source *windPenaltyFile) *WindDependentPenalty {
	if source == nil {
		return nil
	}
	return &WindDependentPenalty{
		LowWind: ManeuverPenalty{
			SpeedRatio: SpeedRatio(source.LowWind.Ratio),
			Duration:   secondsToDuration(source.LowWind.Timer),
		},
		HighWind: ManeuverPenalty{
			SpeedRatio: SpeedRatio(source.HighWind.Ratio),
			Duration:   secondsToDuration(source.HighWind.Timer),
		},
	}
}

func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
