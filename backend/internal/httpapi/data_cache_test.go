package httpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testLandGeoJSON = `{"type":"FeatureCollection","features":[{"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}}]}`

func TestCachedFileSignatureChangesWhenFileChanges(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "weather.grb2")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}

	key, first, err := cachedFileSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	if key != filepath.Clean(path) {
		t.Fatalf("clé obtenue = %q, attendu %q", key, filepath.Clean(path))
	}

	if err := os.WriteFile(path, []byte("second content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, second, err := cachedFileSignature(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("la signature n'a pas changé: avant=%+v après=%+v", first, second)
	}
}

func TestLoadCachedLandReusesAndInvalidates(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "land.geojson")
	if err := os.WriteFile(path, []byte(testLandGeoJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	first, hit, err := loadCachedLand(path)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatal("la première lecture ne devait pas être un succès du cache")
	}
	second, hit, err := loadCachedLand(path)
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("la deuxième lecture devait être un succès du cache")
	}
	if first != second {
		t.Fatal("la deuxième lecture devait réutiliser la contrainte en cache")
	}

	updated := fmt.Sprintf("%s ", testLandGeoJSON)
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, time.Now().Add(time.Second), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	third, hit, err := loadCachedLand(path)
	if err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Fatal("la lecture après modification ne devait pas être un succès du cache")
	}
	if third == second {
		t.Fatal("la contrainte devait être rechargée après modification du fichier")
	}
}
