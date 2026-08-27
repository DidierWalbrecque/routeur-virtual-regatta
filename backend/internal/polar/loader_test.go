package polar

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validPolarJSON = `{
  "_id": 21,
  "label": "Test boat",
  "globalSpeedRatio": 1,
  "iceSpeedRatio": 0.3,
  "autoSailChangeTolerance": 1.014,
  "badSailTolerance": 1.02,
  "maxSpeed": 20,
  "_updatedAt": "2024-07-02T00:00:00Z",
  "foil": {
    "speedRatio": 1.04,
    "twaMin": 80,
    "twaMax": 160,
    "twaMerge": 10,
    "twsMin": 16,
    "twsMax": 35,
    "twsMerge": 5
  },
  "hull": {"speedRatio": 1.003},
  "winch": {
    "lws": 10,
    "hws": 30,
    "tack": {
      "stdTimerSec": 300,
      "stdRatio": 0.5,
      "proTimerSec": 75,
      "proRatio": 0.5,
      "std": {
        "lw": {"ratio": 0.5, "timer": 180},
        "hw": {"ratio": 0.5, "timer": 300}
      },
      "pro": {
        "lw": {"ratio": 0.7, "timer": 180},
        "hw": {"ratio": 0.7, "timer": 300}
      }
    },
    "gybe": {},
    "sailChange": {}
  },
  "tws": [0, 10, 20],
  "twa": [0, 90, 180],
  "sail": [
    {
      "id": 1,
      "name": "Jib",
      "speed": [
        [0, 0, 0],
        [0, 10, 15],
        [0, 8, 12]
      ]
    }
  ]
}`

func TestLoadValidPolar(t *testing.T) {
	polar, err := Load(strings.NewReader(validPolarJSON))
	if err != nil {
		t.Fatalf("Load() a retourné une erreur inattendue: %v", err)
	}

	if polar.BoatID != 21 {
		t.Errorf("BoatID = %d, attendu 21", polar.BoatID)
	}
	if polar.Label != "Test boat" {
		t.Errorf("Label = %q, attendu %q", polar.Label, "Test boat")
	}
	if len(polar.WindSpeeds) != 3 || len(polar.WindAngles) != 3 {
		t.Errorf("dimensions des axes = %dx%d, attendu 3x3", len(polar.WindAngles), len(polar.WindSpeeds))
	}
	if got := polar.Sails[0].Speeds.Values[1][2]; got != 15 {
		t.Errorf("vitesse = %v, attendu 15", got)
	}
	if polar.Winch.Tack.Standard == nil {
		t.Fatal("la pénalité standard détaillée du tack ne doit pas être nil")
	}
	if got := polar.Winch.Tack.Standard.LowWind.Duration; got != 3*time.Minute {
		t.Errorf("durée de pénalité = %v, attendu 3m", got)
	}
	if polar.UpdatedAt.IsZero() {
		t.Error("UpdatedAt ne doit pas être nul")
	}
}

func TestLoadAcceptsLegacyOptionalFields(t *testing.T) {
	legacyJSON := `{
      "_id": 13,
      "label": "Legacy boat",
      "globalSpeedRatio": 1,
      "iceSpeedRatio": 0.3,
      "autoSailChangeTolerance": 1.003,
      "badSailTolerance": 1.01,
      "foil": {},
      "hull": {},
      "winch": {
        "tack": {"stdTimerSec": 300, "stdRatio": 0.5, "proTimerSec": 75, "proRatio": 0.5}
      },
      "tws": [0, 70],
      "twa": [0, 180],
      "sail": [{"id": 1, "name": "Jib", "speed": [[0, 0], [0, 0]]}]
    }`

	polar, err := Load(strings.NewReader(legacyJSON))
	if err != nil {
		t.Fatalf("Load() doit accepter les champs optionnels absents: %v", err)
	}
	if polar.Winch.Tack.Standard != nil {
		t.Error("la pénalité détaillée doit être nil dans un ancien fichier")
	}
	if got := polar.Winch.Tack.LegacyStd.Duration; got != 5*time.Minute {
		t.Errorf("durée historique = %v, attendu 5m", got)
	}
}

func TestLoadUltimSampleFile(t *testing.T) {
	path := filepath.Join("..", "..", "..", "data", "polars", "ultim_bp_11.json")

	polar, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() n'a pas chargé la polaire Ultim: %v", err)
	}

	if got := len(polar.WindSpeeds); got != 18 {
		t.Errorf("nombre de TWS = %d, attendu 18", got)
	}
	if got := len(polar.WindAngles); got != 29 {
		t.Errorf("nombre de TWA = %d, attendu 29", got)
	}
	if got := len(polar.Sails); got != 7 {
		t.Errorf("nombre de voiles = %d, attendu 7", got)
	}
	if got := polar.Sails[4].Speeds.Values[15][8]; got != 36.8 {
		t.Errorf("vitesse Code0 à TWA=90 et TWS=20 = %v, attendu 36.8", got)
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	_, err := Load(strings.NewReader(`{"label":`))
	if err == nil {
		t.Fatal("Load() doit refuser un JSON mal formé")
	}
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
	_, err := Load(strings.NewReader(validPolarJSON + `{}`))
	if err == nil {
		t.Fatal("Load() doit refuser un second objet JSON")
	}
}

func TestValidateRejectsInvalidPolarStructures(t *testing.T) {
	tests := []struct {
		name        string
		jsonReplace string
		replacement string
		errorPart   string
	}{
		{
			name:        "TWS non croissant",
			jsonReplace: `"tws": [0, 10, 20]`,
			replacement: `"tws": [0, 20, 10]`,
			errorPart:   "TWS n'est pas strictement croissant",
		},
		{
			name:        "TWA hors limites",
			jsonReplace: `"twa": [0, 90, 180]`,
			replacement: `"twa": [0, 90, 181]`,
			errorPart:   "compris entre 0 et 180",
		},
		{
			name:        "nombre de lignes incorrect",
			jsonReplace: "[0, 8, 12]",
			replacement: "[0, 8, 12],\n        [0, 7, 11]",
			errorPart:   "possède 4 lignes, attendu 3",
		},
		{
			name:        "nombre de colonnes incorrect",
			jsonReplace: "[0, 10, 15]",
			replacement: "[0, 10]",
			errorPart:   "possède 2 colonnes, attendu 3",
		},
		{
			name:        "vitesse négative",
			jsonReplace: "[0, 10, 15]",
			replacement: "[0, -1, 15]",
			errorPart:   "doit être un nombre fini positif ou nul",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := strings.Replace(validPolarJSON, test.jsonReplace, test.replacement, 1)
			_, err := Load(strings.NewReader(input))
			if err == nil {
				t.Fatal("Load() doit refuser cette structure")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}

func TestValidateRejectsDuplicateSails(t *testing.T) {
	duplicate := strings.Replace(
		validPolarJSON,
		"]\n    }\n  ]",
		`]
    },
    {
      "id": 1,
      "name": "Other",
      "speed": [[0, 0, 0], [0, 1, 1], [0, 1, 1]]
    }
  ]`,
		1,
	)

	_, err := Load(strings.NewReader(duplicate))
	if err == nil {
		t.Fatal("Load() doit refuser deux voiles portant le même identifiant")
	}
	if !strings.Contains(err.Error(), "identifiant de voile dupliqué") {
		t.Errorf("erreur inattendue: %v", err)
	}
}
