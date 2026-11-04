package httpapi

import "testing"

func TestBoatStaminaCoefficient(t *testing.T) {
	if got := boatStaminaCoefficient(21); got != 1.5 {
		t.Errorf("Ultim BP XI (21) = %.2f, attendu 1.5", got)
	}
	if got := boatStaminaCoefficient(3); got != 1 {
		t.Errorf("Class 40 (3) = %.2f, attendu 1 (défaut)", got)
	}
	if got := boatStaminaCoefficient(999); got != 1 {
		t.Errorf("bateau inconnu = %.2f, attendu 1", got)
	}
}
