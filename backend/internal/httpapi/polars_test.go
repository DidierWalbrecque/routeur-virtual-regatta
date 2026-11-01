package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPolarsHandlerListsJSONFilesSortedWithDefault(t *testing.T) {
	directory := t.TempDir()
	contents := map[string]string{
		"Zeta.json":  "{}",
		"alpha.json": `{"_id": 7, "label": "mono/alpha"}`,
		"notes.txt":  "{}",
	}
	for name, content := range contents {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	previous := polarsDirectory
	polarsDirectory = directory
	t.Cleanup(func() { polarsDirectory = previous })

	recorder := httptest.NewRecorder()
	NewHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/polars", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("statut %d, corps %s", recorder.Code, recorder.Body.String())
	}
	var body polarsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Polars) != 2 || body.Polars[0].Label != "alpha.json" || body.Polars[1].Label != "Zeta.json" {
		t.Fatalf("polaires inattendues: %+v", body.Polars)
	}
	if body.Polars[0].Value != "../data/polars/alpha.json" {
		t.Fatalf("valeur inattendue: %q", body.Polars[0].Value)
	}
	if body.Polars[0].ID != 7 || body.Polars[0].Boat != "mono/alpha" {
		t.Fatalf("identité du bateau attendue (7, mono/alpha), obtenu %+v", body.Polars[0])
	}
	if body.Polars[1].ID != 0 || body.Polars[1].Boat != "" {
		t.Fatalf("aucune identité attendue pour un fichier sans _id, obtenu %+v", body.Polars[1])
	}
	if body.Default != "../data/polars/alpha.json" {
		t.Fatalf("défaut attendu: première polaire, obtenu %q", body.Default)
	}
}
