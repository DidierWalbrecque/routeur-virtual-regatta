package land

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	NaturalEarthLandFileName = "natural-earth-10m-land.geojson"

	defaultNaturalEarthLandURL = "https://raw.githubusercontent.com/" +
		"nvkelso/natural-earth-vector/" +
		"ca96624a56bd078437bca8184e78163e5039ad19/" +
		"geojson/ne_10m_land.geojson"
	defaultNaturalEarthMaxDownloadSize = int64(256 << 20)
)

// NaturalEarthDownload describes one validated local land dataset.
type NaturalEarthDownload struct {
	Path         string
	Size         int64
	PolygonCount int
	Reused       bool
}

// NaturalEarthDownloader downloads the pinned Natural Earth 1:10m land
// GeoJSON independently from the routing engine.
type NaturalEarthDownloader struct {
	HTTPClient      *http.Client
	SourceURL       string
	MaxDownloadSize int64
	validateFile    func(string) (int, error)
}

// NewNaturalEarthDownloader creates a downloader with a 256 MiB limit.
func NewNaturalEarthDownloader() *NaturalEarthDownloader {
	return &NaturalEarthDownloader{
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		SourceURL:       defaultNaturalEarthLandURL,
		MaxDownloadSize: defaultNaturalEarthMaxDownloadSize,
		validateFile:    validateNaturalEarthFile,
	}
}

// Download stores a complete validated GeoJSON in outputDirectory. An existing
// valid file is reused. A partial or invalid download never receives the final
// file name.
func (downloader *NaturalEarthDownloader) Download(
	ctx context.Context,
	outputDirectory string,
) (NaturalEarthDownload, error) {
	if downloader == nil {
		return NaturalEarthDownload{}, errors.New(
			"land: téléchargeur Natural Earth non initialisé",
		)
	}
	if ctx == nil {
		return NaturalEarthDownload{}, errors.New(
			"land: contexte de téléchargement requis",
		)
	}
	if err := ctx.Err(); err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: téléchargement annulé: %w",
			err,
		)
	}
	if strings.TrimSpace(outputDirectory) == "" {
		return NaturalEarthDownload{}, errors.New(
			"land: dossier de destination requis",
		)
	}
	if downloader.validateFile == nil {
		return NaturalEarthDownload{}, errors.New(
			"land: validation du fichier téléchargé requise",
		)
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: création du dossier %q impossible: %w",
			outputDirectory,
			err,
		)
	}

	destination := filepath.Join(
		outputDirectory,
		NaturalEarthLandFileName,
	)
	if info, err := os.Stat(destination); err == nil {
		if info.IsDir() {
			return NaturalEarthDownload{}, fmt.Errorf(
				"land: la destination %q est un dossier",
				destination,
			)
		}
		polygonCount, err := downloader.validateFile(destination)
		if err != nil {
			return NaturalEarthDownload{}, fmt.Errorf(
				"land: fichier existant %q invalide: %w",
				destination,
				err,
			)
		}
		return NaturalEarthDownload{
			Path:         destination,
			Size:         info.Size(),
			PolygonCount: polygonCount,
			Reused:       true,
		}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: contrôle de %q impossible: %w",
			destination,
			err,
		)
	}

	if downloader.HTTPClient == nil {
		return NaturalEarthDownload{}, errors.New(
			"land: client HTTP Natural Earth requis",
		)
	}
	if strings.TrimSpace(downloader.SourceURL) == "" {
		return NaturalEarthDownload{}, errors.New(
			"land: URL Natural Earth requise",
		)
	}
	if downloader.MaxDownloadSize <= 0 {
		return NaturalEarthDownload{}, errors.New(
			"land: limite de téléchargement invalide",
		)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		downloader.SourceURL,
		nil,
	)
	if err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: création de la requête Natural Earth impossible: %w",
			err,
		)
	}
	request.Header.Set(
		"User-Agent",
		"routeur-virtual-regatta/landdownload",
	)
	response, err := downloader.HTTPClient.Do(request)
	if err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: téléchargement Natural Earth impossible: %w",
			err,
		)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: Natural Earth répond %s: %s",
			response.Status,
			strings.TrimSpace(string(detail)),
		)
	}

	temporary, err := os.CreateTemp(
		outputDirectory,
		".natural-earth-download-*.geojson",
	)
	if err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: création du fichier temporaire impossible: %w",
			err,
		)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	written, copyErr := io.Copy(
		temporary,
		io.LimitReader(response.Body, downloader.MaxDownloadSize+1),
	)
	closeErr := temporary.Close()
	if copyErr != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: écriture du téléchargement impossible: %w",
			copyErr,
		)
	}
	if closeErr != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: fermeture du téléchargement impossible: %w",
			closeErr,
		)
	}
	if written > downloader.MaxDownloadSize {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: téléchargement supérieur à la limite de %d octets",
			downloader.MaxDownloadSize,
		)
	}
	if written == 0 {
		return NaturalEarthDownload{}, errors.New(
			"land: Natural Earth a renvoyé un fichier vide",
		)
	}

	polygonCount, err := downloader.validateFile(temporaryPath)
	if err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: fichier Natural Earth téléchargé invalide: %w",
			err,
		)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return NaturalEarthDownload{}, fmt.Errorf(
			"land: installation de %q impossible: %w",
			destination,
			err,
		)
	}
	return NaturalEarthDownload{
		Path:         destination,
		Size:         written,
		PolygonCount: polygonCount,
	}, nil
}

func validateNaturalEarthFile(path string) (int, error) {
	constraint, err := LoadGeoJSONFile(path)
	if err != nil {
		return 0, err
	}
	return constraint.PolygonCount(), nil
}
