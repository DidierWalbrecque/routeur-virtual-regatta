package routing

import (
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/simulation"
)

func TestPracticalStabilityScore(t *testing.T) {
	tests := []struct {
		name   string
		speeds []polar.BoatSpeed
		want   float64
	}{
		{name: "aucun pas", want: 0},
		{name: "vitesse constante", speeds: []polar.BoatSpeed{8, 8, 8}, want: 1},
		{name: "vitesse variable", speeds: []polar.BoatSpeed{8, 12}, want: 0.8},
		{name: "vitesses nulles", speeds: []polar.BoatSpeed{0, 0}, want: 1},
		{name: "variabilité extrême", speeds: []polar.BoatSpeed{0, 10}, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := simulation.Result{}
			for _, speed := range test.speeds {
				result.Steps = append(result.Steps, simulation.StepResult{
					BestSail: polar.SailPerformance{Speed: speed},
				})
			}
			assertClose(t, "score de stabilité", practicalStabilityScore(result), test.want)
		})
	}
}
