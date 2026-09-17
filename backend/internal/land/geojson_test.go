package land

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/routing"
)

func TestDecodeGeoJSONPolygonAndMultiPolygon(t *testing.T) {
	const document = `{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"properties": {"name": "island"},
				"geometry": {
					"type": "Polygon",
					"coordinates": [[[0,0],[1,0],[1,1],[0,1],[0,0]]]
				}
			},
			{
				"type": "Feature",
				"properties": {},
				"geometry": {
					"type": "MultiPolygon",
					"coordinates": [
						[[[2,0],[3,0],[3,1],[2,1],[2,0]]],
						[[[4,0],[5,0],[5,1],[4,1],[4,0]]]
					]
				}
			}
		]
	}`

	constraint, err := DecodeGeoJSON(strings.NewReader(document))
	if err != nil {
		t.Fatalf("décodage refusé: %v", err)
	}
	if constraint.PolygonCount() != 3 {
		t.Errorf(
			"polygones = %d, attendus 3",
			constraint.PolygonCount(),
		)
	}
	var _ routing.Constraint = constraint
}

func TestLoadGeoJSONFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "land.geojson")
	err := os.WriteFile(
		path,
		[]byte(`{
			"type":"FeatureCollection",
			"features":[{
				"type":"Feature",
				"geometry":{
					"type":"Polygon",
					"coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]
				}
			}]
		}`),
		0o600,
	)
	if err != nil {
		t.Fatalf("fixture impossible: %v", err)
	}

	constraint, err := LoadGeoJSONFile(path)
	if err != nil {
		t.Fatalf("chargement refusé: %v", err)
	}
	if constraint.PolygonCount() != 1 {
		t.Errorf(
			"polygones = %d, attendu 1",
			constraint.PolygonCount(),
		)
	}
}

func TestDecodeGeoJSONRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name     string
		document string
		wantErr  string
	}{
		{
			name:     "collection vide",
			document: `{"type":"FeatureCollection","features":[]}`,
			wantErr:  "aucune entité",
		},
		{
			name: "type racine invalide",
			document: `{
				"type":"Feature",
				"features":[]
			}`,
			wantErr: "FeatureCollection",
		},
		{
			name: "géométrie absente",
			document: `{
				"type":"FeatureCollection",
				"features":[{"type":"Feature","geometry":null}]
			}`,
			wantErr: "géométrie absente",
		},
		{
			name: "géométrie non prise en charge",
			document: `{
				"type":"FeatureCollection",
				"features":[{
					"type":"Feature",
					"geometry":{
						"type":"LineString",
						"coordinates":[[0,0],[1,1]]
					}
				}]
			}`,
			wantErr: "non pris en charge",
		},
		{
			name: "anneau ouvert",
			document: `{
				"type":"FeatureCollection",
				"features":[{
					"type":"Feature",
					"geometry":{
						"type":"Polygon",
						"coordinates":[[[0,0],[1,0],[1,1],[0,1]]]
					}
				}]
			}`,
			wantErr: "identiques",
		},
		{
			name: "longitude invalide",
			document: `{
				"type":"FeatureCollection",
				"features":[{
					"type":"Feature",
					"geometry":{
						"type":"Polygon",
						"coordinates":[[[181,0],[1,0],[1,1],[181,0]]]
					}
				}]
			}`,
			wantErr: "longitude",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeGeoJSON(strings.NewReader(test.document))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("erreur = %v, doit contenir %q", err, test.wantErr)
			}
		})
	}
}

func TestDecodeGeoJSONRejectsTrailingContent(t *testing.T) {
	document := `{"type":"FeatureCollection","features":[
		{"type":"Feature","geometry":{"type":"Polygon","coordinates":[
			[[0,0],[1,0],[1,1],[0,1],[0,0]]
		]}}
	]} {}`
	_, err := DecodeGeoJSON(strings.NewReader(document))
	if err == nil || !strings.Contains(err.Error(), "inattendu") {
		t.Fatalf("erreur = %v, contenu final invalide attendu", err)
	}
}
