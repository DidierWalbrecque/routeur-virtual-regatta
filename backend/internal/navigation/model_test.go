package navigation

import (
	"math"
	"strings"
	"testing"
)

func TestPositionValidate(t *testing.T) {
	tests := []struct {
		name       string
		position   Position
		shouldFail bool
		errorPart  string
	}{
		{
			name:     "origine",
			position: Position{},
		},
		{
			name: "limites incluses",
			position: Position{
				Latitude:  90,
				Longitude: -180,
			},
		},
		{
			name: "latitude trop basse",
			position: Position{
				Latitude: -90.1,
			},
			shouldFail: true,
			errorPart:  "latitude",
		},
		{
			name: "latitude trop haute",
			position: Position{
				Latitude: 90.1,
			},
			shouldFail: true,
			errorPart:  "latitude",
		},
		{
			name: "longitude trop basse",
			position: Position{
				Longitude: -180.1,
			},
			shouldFail: true,
			errorPart:  "longitude",
		},
		{
			name: "longitude trop haute",
			position: Position{
				Longitude: 180.1,
			},
			shouldFail: true,
			errorPart:  "longitude",
		},
		{
			name: "latitude non finie",
			position: Position{
				Latitude: Latitude(math.NaN()),
			},
			shouldFail: true,
			errorPart:  "latitude",
		},
		{
			name: "longitude non finie",
			position: Position{
				Longitude: Longitude(math.Inf(1)),
			},
			shouldFail: true,
			errorPart:  "longitude",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.position.Validate()
			if !test.shouldFail {
				if err != nil {
					t.Fatalf("Validate() a retourné une erreur inattendue: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Validate() doit refuser cette position")
			}
			if !strings.Contains(err.Error(), test.errorPart) {
				t.Errorf("erreur = %q, doit contenir %q", err, test.errorPart)
			}
		})
	}
}
