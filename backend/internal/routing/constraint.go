package routing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
)

// RouteSegment describes one simulated movement submitted to route constraints.
type RouteSegment struct {
	From       navigation.Position
	To         navigation.Position
	StartedAt  time.Time
	FinishedAt time.Time
}

// Validate checks that the segment contains valid positions and a positive
// chronological interval.
func (segment RouteSegment) Validate() error {
	if err := segment.From.Validate(); err != nil {
		return fmt.Errorf("routing: départ du segment invalide: %w", err)
	}
	if err := segment.To.Validate(); err != nil {
		return fmt.Errorf("routing: arrivée du segment invalide: %w", err)
	}
	if segment.StartedAt.IsZero() {
		return errors.New("routing: instant de départ du segment requis")
	}
	if segment.FinishedAt.IsZero() {
		return errors.New("routing: instant d'arrivée du segment requis")
	}
	if !segment.FinishedAt.After(segment.StartedAt) {
		return errors.New(
			"routing: l'instant d'arrivée du segment doit suivre son départ",
		)
	}
	return nil
}

// Constraint accepts or rejects one simulated route segment.
type Constraint interface {
	Allows(context.Context, RouteSegment) (bool, error)
}

// ConstraintFunc adapts a function to the Constraint interface.
type ConstraintFunc func(context.Context, RouteSegment) (bool, error)

// Allows calls the adapted constraint function.
func (constraint ConstraintFunc) Allows(
	ctx context.Context,
	segment RouteSegment,
) (bool, error) {
	if constraint == nil {
		return false, errors.New("routing: fonction de contrainte requise")
	}
	return constraint(ctx, segment)
}

func segmentAllowed(
	ctx context.Context,
	constraints []Constraint,
	segment RouteSegment,
) (bool, error) {
	if ctx == nil {
		return false, errors.New("routing: contexte de contrainte requis")
	}
	if err := segment.Validate(); err != nil {
		return false, err
	}

	for index, constraint := range constraints {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf(
				"routing: vérification des contraintes interrompue: %w",
				err,
			)
		}
		if constraint == nil {
			return false, fmt.Errorf(
				"routing: contrainte %d absente",
				index+1,
			)
		}
		allowed, err := constraint.Allows(ctx, segment)
		if err != nil {
			return false, fmt.Errorf(
				"routing: contrainte %d impossible: %w",
				index+1,
				err,
			)
		}
		if !allowed {
			return false, nil
		}
	}
	return true, nil
}
