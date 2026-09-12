package simulation

import (
	"math"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"
	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/polar"
)

func TestDetectManeuver(t *testing.T) {
	tests := []struct {
		name       string
		transition ManeuverTransition
		want       ManeuverState
	}{
		{
			name: "changement de cap simple",
			transition: validManeuverTransition(
				navigation.TackStarboard,
				navigation.TackStarboard,
				60,
				75,
				1,
				1,
			),
			want: ManeuverState{Kind: ManeuverNone},
		},
		{
			name: "virement",
			transition: validManeuverTransition(
				navigation.TackPort,
				navigation.TackStarboard,
				40,
				50,
				1,
				1,
			),
			want: ManeuverState{Kind: ManeuverTack},
		},
		{
			name: "empannage",
			transition: validManeuverTransition(
				navigation.TackStarboard,
				navigation.TackPort,
				130,
				140,
				1,
				1,
			),
			want: ManeuverState{Kind: ManeuverGybe},
		},
		{
			name: "changement de voile seul",
			transition: validManeuverTransition(
				navigation.TackPort,
				navigation.TackPort,
				80,
				90,
				1,
				3,
			),
			want: ManeuverState{
				Kind:        ManeuverNone,
				SailChanged: true,
			},
		},
		{
			name: "virement et changement de voile",
			transition: validManeuverTransition(
				navigation.TackPort,
				navigation.TackStarboard,
				30,
				45,
				1,
				3,
			),
			want: ManeuverState{
				Kind:        ManeuverTack,
				SailChanged: true,
			},
		},
		{
			name: "amure précédente indéfinie",
			transition: validManeuverTransition(
				navigation.TackUndefined,
				navigation.TackStarboard,
				0,
				30,
				1,
				1,
			),
			want: ManeuverState{Kind: ManeuverNone},
		},
		{
			name: "nouvelle amure indéfinie avec changement de voile",
			transition: validManeuverTransition(
				navigation.TackPort,
				navigation.TackUndefined,
				30,
				180,
				1,
				2,
			),
			want: ManeuverState{
				Kind:        ManeuverNone,
				SailChanged: true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DetectManeuver(test.transition)
			if err != nil {
				t.Fatalf("détection refusée: %v", err)
			}
			if got != test.want {
				t.Errorf("manœuvre = %+v, attendue %+v", got, test.want)
			}
		})
	}
}

func TestDetectManeuverRejectsAmbiguousTransition(t *testing.T) {
	_, err := DetectManeuver(validManeuverTransition(
		navigation.TackPort,
		navigation.TackStarboard,
		90,
		90,
		1,
		1,
	))
	if err == nil || !strings.Contains(err.Error(), "ambiguë") {
		t.Fatalf("erreur = %v, doit signaler une transition ambiguë", err)
	}
}

func TestDetectManeuverRejectsInvalidInputs(t *testing.T) {
	valid := validManeuverTransition(
		navigation.TackPort,
		navigation.TackStarboard,
		40,
		50,
		1,
		2,
	)
	tests := []struct {
		name    string
		mutate  func(*ManeuverTransition)
		wantErr string
	}{
		{
			name: "amure précédente",
			mutate: func(transition *ManeuverTransition) {
				transition.PreviousTack = "left"
			},
			wantErr: "amure précédente",
		},
		{
			name: "nouvelle amure",
			mutate: func(transition *ManeuverTransition) {
				transition.NextTack = "right"
			},
			wantErr: "nouvelle amure",
		},
		{
			name: "TWA précédent",
			mutate: func(transition *ManeuverTransition) {
				transition.PreviousWindAngle =
					navigation.TrueWindAngle(math.NaN())
			},
			wantErr: "TWA précédent",
		},
		{
			name: "nouveau TWA",
			mutate: func(transition *ManeuverTransition) {
				transition.NextWindAngle = 181
			},
			wantErr: "nouveau TWA",
		},
		{
			name: "voile précédente",
			mutate: func(transition *ManeuverTransition) {
				transition.PreviousSailID = 0
			},
			wantErr: "voile précédent",
		},
		{
			name: "nouvelle voile",
			mutate: func(transition *ManeuverTransition) {
				transition.NextSailID = -1
			},
			wantErr: "nouvel identifiant",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transition := valid
			test.mutate(&transition)
			_, err := DetectManeuver(transition)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func validManeuverTransition(
	previousTack navigation.Tack,
	nextTack navigation.Tack,
	previousWindAngle navigation.TrueWindAngle,
	nextWindAngle navigation.TrueWindAngle,
	previousSailID,
	nextSailID polar.SailID,
) ManeuverTransition {
	return ManeuverTransition{
		PreviousTack:      previousTack,
		NextTack:          nextTack,
		PreviousWindAngle: previousWindAngle,
		NextWindAngle:     nextWindAngle,
		PreviousSailID:    previousSailID,
		NextSailID:        nextSailID,
	}
}
