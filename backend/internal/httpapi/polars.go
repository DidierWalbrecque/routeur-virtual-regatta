package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// polarsDirectory contient les polaires JSON, relativement au dossier backend.
var polarsDirectory = "../data/polars"

const defaultPolarValue = "../data/polars/ultim_bp_11.json"

type polarOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
	// ID et Boat viennent du fichier : identifiant et nom du bateau dans le jeu.
	ID   int    `json:"id,omitempty"`
	Boat string `json:"boat,omitempty"`
}

// polarIdentity est la partie du fichier de polaire qui identifie le bateau.
type polarIdentity struct {
	ID    int    `json:"_id"`
	Label string `json:"label"`
}

type polarsResponse struct {
	Polars  []polarOption `json:"polars"`
	Default string        `json:"default"`
}

// polarsHandler liste les polaires disponibles pour le formulaire de calcul.
func polarsHandler(response http.ResponseWriter, _ *http.Request) {
	entries, err := os.ReadDir(polarsDirectory)
	if err != nil && !os.IsNotExist(err) {
		writeJSONError(response, http.StatusInternalServerError, "lecture des polaires impossible: "+err.Error())
		return
	}
	options := make([]polarOption, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			continue
		}
		option := polarOption{
			Label: entry.Name(),
			Value: "../data/polars/" + entry.Name(),
		}
		if content, readErr := os.ReadFile(filepath.Join(polarsDirectory, entry.Name())); readErr == nil {
			var identity polarIdentity
			if json.Unmarshal(content, &identity) == nil {
				option.ID = identity.ID
				option.Boat = identity.Label
			}
		}
		options = append(options, option)
	}
	sort.Slice(options, func(left, right int) bool {
		return strings.ToLower(options[left].Label) < strings.ToLower(options[right].Label)
	})

	defaultValue := defaultPolarValue
	found := false
	for _, option := range options {
		if option.Value == defaultPolarValue {
			found = true
			break
		}
	}
	if !found && len(options) > 0 {
		defaultValue = options[0].Value
	}
	writeJSON(response, http.StatusOK, polarsResponse{Polars: options, Default: defaultValue})
}
