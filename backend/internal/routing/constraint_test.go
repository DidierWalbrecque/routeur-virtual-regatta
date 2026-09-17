package routing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

func TestSegmentAllowedAcceptsEveryConstraint(t *testing.T) {
	segment := validConstraintTestSegment()
	calls := 0
	constraints := []Constraint{
		ConstraintFunc(func(
			context.Context,
			RouteSegment,
		) (bool, error) {
			calls++
			return true, nil
		}),
		ConstraintFunc(func(
			context.Context,
			RouteSegment,
		) (bool, error) {
			calls++
			return true, nil
		}),
	}

	allowed, err := segmentAllowed(context.Background(), constraints, segment)
	if err != nil {
		t.Fatalf("vérification refusée: %v", err)
	}
	if !allowed {
		t.Fatal("segment refusé")
	}
	if calls != 2 {
		t.Errorf("appels = %d, attendus 2", calls)
	}
}

func TestSegmentAllowedStopsAtFirstRejection(t *testing.T) {
	segment := validConstraintTestSegment()
	calls := 0
	constraints := []Constraint{
		ConstraintFunc(func(
			context.Context,
			RouteSegment,
		) (bool, error) {
			calls++
			return false, nil
		}),
		ConstraintFunc(func(
			context.Context,
			RouteSegment,
		) (bool, error) {
			calls++
			return true, nil
		}),
	}

	allowed, err := segmentAllowed(context.Background(), constraints, segment)
	if err != nil {
		t.Fatalf("vérification refusée: %v", err)
	}
	if allowed {
		t.Fatal("segment autorisé malgré le rejet")
	}
	if calls != 1 {
		t.Errorf("appels = %d, attendu 1", calls)
	}
}

func TestSegmentAllowedPropagatesConstraintError(t *testing.T) {
	_, err := segmentAllowed(
		context.Background(),
		[]Constraint{
			ConstraintFunc(func(
				context.Context,
				RouteSegment,
			) (bool, error) {
				return false, errors.New("données indisponibles")
			}),
		},
		validConstraintTestSegment(),
	)
	if err == nil || !strings.Contains(err.Error(), "contrainte 1") {
		t.Fatalf("erreur = %v, contrainte fautive attendue", err)
	}
}

func TestSegmentAllowedRejectsInvalidInputs(t *testing.T) {
	valid := validConstraintTestSegment()
	tests := []struct {
		name        string
		ctx         context.Context
		constraints []Constraint
		segment     RouteSegment
		wantErr     string
	}{
		{
			name:    "contexte absent",
			segment: valid,
			wantErr: "contexte",
		},
		{
			name:        "contrainte absente",
			ctx:         context.Background(),
			constraints: []Constraint{nil},
			segment:     valid,
			wantErr:     "contrainte 1",
		},
		{
			name:    "position invalide",
			ctx:     context.Background(),
			segment: RouteSegment{
				From:       navigation.Position{Latitude: 91},
				To:         valid.To,
				StartedAt:  valid.StartedAt,
				FinishedAt: valid.FinishedAt,
			},
			wantErr: "départ",
		},
		{
			name:    "ordre temporel invalide",
			ctx:     context.Background(),
			segment: RouteSegment{
				From:       valid.From,
				To:         valid.To,
				StartedAt:  valid.StartedAt,
				FinishedAt: valid.StartedAt,
			},
			wantErr: "doit suivre",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := segmentAllowed(
				test.ctx,
				test.constraints,
				test.segment,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func validConstraintTestSegment() RouteSegment {
	startedAt := time.Date(2026, 9, 5, 7, 30, 0, 0, time.UTC)
	return RouteSegment{
		From:       navigation.Position{Latitude: 13, Longitude: 121},
		To:         navigation.Position{Latitude: 14, Longitude: 120},
		StartedAt:  startedAt,
		FinishedAt: startedAt.Add(10 * time.Minute),
	}
}
