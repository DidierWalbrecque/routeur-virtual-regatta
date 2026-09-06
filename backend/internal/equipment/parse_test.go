package equipment

import "testing"

func TestParseEquipment(t *testing.T) {
	tests := []struct {
		input string
		want  Equipment
	}{
		{input: "light", want: Light},
		{input: " HEAVY ", want: Heavy},
		{input: "code0", want: Code0},
		{input: "Foils", want: Foils},
		{input: "hull_polish", want: HullPolish},
		{input: "pro_winches", want: ProWinches},
	}

	for _, test := range tests {
		got, err := Parse(test.input)
		if err != nil {
			t.Fatalf("Parse(%q) a retourné une erreur inattendue: %v", test.input, err)
		}
		if got != test.want {
			t.Errorf("Parse(%q) = %q, attendu %q", test.input, got, test.want)
		}
	}
}

func TestParseEquipmentRejectsUnknownValue(t *testing.T) {
	if _, err := Parse("radio"); err == nil {
		t.Fatal("Parse() doit refuser une option inconnue")
	}
}

func TestParseEquipmentSet(t *testing.T) {
	set, err := ParseSet("code0, Foils,code0")
	if err != nil {
		t.Fatalf("ParseSet() a retourné une erreur: %v", err)
	}
	if !set.Has(Code0) || !set.Has(Foils) {
		t.Fatalf("ensemble incomplet: %v", set)
	}
	if set.Has(Light) {
		t.Fatalf("équipement inattendu dans %v", set)
	}
}

func TestParseEquipmentSetAcceptsEmptyValue(t *testing.T) {
	set, err := ParseSet("  ")
	if err != nil {
		t.Fatalf("ParseSet() a retourné une erreur: %v", err)
	}
	if set != 0 {
		t.Fatalf("ensemble = %v, attendu vide", set)
	}
}

func TestParseEquipmentSetRejectsUnknownValue(t *testing.T) {
	if _, err := ParseSet("code0,radio"); err == nil {
		t.Fatal("ParseSet() doit refuser une option inconnue")
	}
}
