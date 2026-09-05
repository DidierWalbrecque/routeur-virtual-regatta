package weather

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultNOAAGFSEndpoint = "https://nomads.ncep.noaa.gov/cgi-bin/filter_gfs_0p25.pl"
	defaultMaxDownloadSize = int64(64 << 20)
	gfsDateLayout           = "20060102"
)

// GFSForecastRequest identifie une échéance du modèle NOAA GFS 0,25°.
type GFSForecastRequest struct {
	Date         time.Time
	Cycle        int
	ForecastHour int
}

// NewGFSForecastRequest construit une demande à partir d'une date YYYYMMDD,
// d'un cycle 00, 06, 12 ou 18 et d'une échéance comprise entre f000 et f384.
func NewGFSForecastRequest(
	date string,
	cycle,
	forecastHour int,
) (GFSForecastRequest, error) {
	runDate, err := time.Parse(gfsDateLayout, date)
	if err != nil {
		return GFSForecastRequest{}, fmt.Errorf(
			"weather: date GFS %q invalide, format YYYYMMDD attendu: %w",
			date,
			err,
		)
	}
	request := GFSForecastRequest{
		Date:         runDate,
		Cycle:        cycle,
		ForecastHour: forecastHour,
	}
	if err := request.Validate(); err != nil {
		return GFSForecastRequest{}, err
	}
	return request, nil
}

// Validate vérifie les limites documentées du produit GFS.
func (request GFSForecastRequest) Validate() error {
	if request.Date.IsZero() {
		return errors.New("weather: date du run GFS requise")
	}
	normalizedDate := time.Date(
		request.Date.Year(),
		request.Date.Month(),
		request.Date.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)
	if !request.Date.Equal(normalizedDate) {
		return errors.New(
			"weather: la date du run GFS doit être exprimée à minuit UTC",
		)
	}
	switch request.Cycle {
	case 0, 6, 12, 18:
	default:
		return fmt.Errorf(
			"weather: cycle GFS %02d invalide, valeurs permises: 00, 06, 12, 18",
			request.Cycle,
		)
	}
	if request.ForecastHour < 0 || request.ForecastHour > 384 {
		return fmt.Errorf(
			"weather: échéance GFS f%03d hors limites [f000, f384]",
			request.ForecastHour,
		)
	}
	return nil
}

// ValidAt retourne l'instant couvert par l'échéance.
func (request GFSForecastRequest) ValidAt() time.Time {
	return request.Date.
		Add(time.Duration(request.Cycle) * time.Hour).
		Add(time.Duration(request.ForecastHour) * time.Hour)
}

// SourceFileName retourne le nom employé par NOMADS.
func (request GFSForecastRequest) SourceFileName() string {
	return fmt.Sprintf(
		"gfs.t%02dz.pgrb2.0p25.f%03d",
		request.Cycle,
		request.ForecastHour,
	)
}

// LocalFileName retourne le nom stable employé dans data/weather.
func (request GFSForecastRequest) LocalFileName() string {
	return fmt.Sprintf(
		"gfs.%s%02d.f%03d.grb2",
		request.Date.Format(gfsDateLayout),
		request.Cycle,
		request.ForecastHour,
	)
}

// NOAAGFSDownloader télécharge uniquement UGRD et VGRD à 10 mètres depuis le
// filtre officiel NOMADS. Il est séparé du WeatherProvider utilisé par le
// moteur de routage.
type NOAAGFSDownloader struct {
	HTTPClient      *http.Client
	Endpoint        string
	MaxDownloadSize int64
	validateFile    func(string, GFSForecastRequest) error
}

// NewNOAAGFSDownloader crée un téléchargeur avec une limite de 64 Mio.
func NewNOAAGFSDownloader() *NOAAGFSDownloader {
	return &NOAAGFSDownloader{
		HTTPClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
		Endpoint:        defaultNOAAGFSEndpoint,
		MaxDownloadSize: defaultMaxDownloadSize,
		validateFile:    validateDownloadedWindGrid,
	}
}

// Download télécharge une échéance dans outputDirectory. Un fichier existant
// et valide est réutilisé. Un nouveau fichier n'apparaît à son emplacement
// final qu'après téléchargement et validation complets.
func (downloader *NOAAGFSDownloader) Download(
	ctx context.Context,
	request GFSForecastRequest,
	outputDirectory string,
) (string, error) {
	if downloader == nil {
		return "", errors.New("weather: téléchargeur NOAA non initialisé")
	}
	if ctx == nil {
		return "", errors.New("weather: contexte de téléchargement requis")
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("weather: téléchargement annulé: %w", err)
	}
	if err := request.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(outputDirectory) == "" {
		return "", errors.New("weather: dossier de destination requis")
	}
	if downloader.validateFile == nil {
		return "", errors.New("weather: validation du fichier téléchargé requise")
	}

	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return "", fmt.Errorf(
			"weather: création du dossier %q impossible: %w",
			outputDirectory,
			err,
		)
	}

	destination := filepath.Join(outputDirectory, request.LocalFileName())
	if info, err := os.Stat(destination); err == nil {
		if info.IsDir() {
			return "", fmt.Errorf(
				"weather: la destination %q est un dossier",
				destination,
			)
		}
		if err := downloader.validateFile(destination, request); err != nil {
			return "", fmt.Errorf(
				"weather: fichier existant %q invalide: %w",
				destination,
				err,
			)
		}
		return destination, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf(
			"weather: contrôle de %q impossible: %w",
			destination,
			err,
		)
	}

	if downloader.HTTPClient == nil {
		return "", errors.New("weather: client HTTP NOAA requis")
	}
	if strings.TrimSpace(downloader.Endpoint) == "" {
		return "", errors.New("weather: URL du service NOAA requise")
	}
	if downloader.MaxDownloadSize <= 0 {
		return "", errors.New("weather: limite de téléchargement invalide")
	}

	sourceURL, err := downloader.forecastURL(request)
	if err != nil {
		return "", err
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		sourceURL,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf(
			"weather: création de la requête NOAA impossible: %w",
			err,
		)
	}
	response, err := downloader.HTTPClient.Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf(
			"weather: téléchargement NOAA impossible: %w",
			err,
		)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return "", fmt.Errorf(
			"weather: NOAA répond %s: %s",
			response.Status,
			strings.TrimSpace(string(detail)),
		)
	}

	temporary, err := os.CreateTemp(
		outputDirectory,
		".gfs-download-*.grb2",
	)
	if err != nil {
		return "", fmt.Errorf(
			"weather: création du fichier temporaire impossible: %w",
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
		return "", fmt.Errorf(
			"weather: écriture du téléchargement NOAA impossible: %w",
			copyErr,
		)
	}
	if closeErr != nil {
		return "", fmt.Errorf(
			"weather: fermeture du téléchargement NOAA impossible: %w",
			closeErr,
		)
	}
	if written > downloader.MaxDownloadSize {
		return "", fmt.Errorf(
			"weather: téléchargement NOAA supérieur à la limite de %d octets",
			downloader.MaxDownloadSize,
		)
	}
	if written == 0 {
		return "", errors.New("weather: NOAA a renvoyé un fichier vide")
	}

	if err := downloader.validateFile(temporaryPath, request); err != nil {
		return "", fmt.Errorf(
			"weather: fichier NOAA téléchargé invalide: %w",
			err,
		)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return "", fmt.Errorf(
			"weather: installation de %q impossible: %w",
			destination,
			err,
		)
	}

	return destination, nil
}

func (downloader *NOAAGFSDownloader) forecastURL(
	request GFSForecastRequest,
) (string, error) {
	endpoint, err := url.Parse(downloader.Endpoint)
	if err != nil {
		return "", fmt.Errorf(
			"weather: URL NOAA %q invalide: %w",
			downloader.Endpoint,
			err,
		)
	}

	query := endpoint.Query()
	query.Set("file", request.SourceFileName())
	query.Set("lev_10_m_above_ground", "on")
	query.Set("var_UGRD", "on")
	query.Set("var_VGRD", "on")
	query.Set("subregion", "")
	query.Set("leftlon", "0")
	query.Set("rightlon", "360")
	query.Set("toplat", "90")
	query.Set("bottomlat", "-90")
	query.Set(
		"dir",
		fmt.Sprintf(
			"/gfs.%s/%02d/atmos",
			request.Date.Format(gfsDateLayout),
			request.Cycle,
		),
	)
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func validateDownloadedWindGrid(
	path string,
	request GFSForecastRequest,
) error {
	grid, err := LoadGRIB2WindGrid(path)
	if err != nil {
		return err
	}
	if !grid.ValidAt.Equal(request.ValidAt()) {
		return fmt.Errorf(
			"échéance décodée %s différente de l'échéance demandée %s",
			grid.ValidAt.Format(time.RFC3339),
			request.ValidAt().Format(time.RFC3339),
		)
	}
	return nil
}
