package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gabehf/koito/internal/catalog"
	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/mbz"
	"github.com/gabehf/koito/internal/utils"
)

type MalojaExport struct {
	Scrobbles []MalojaExportItem `json:"scrobbles"`
}
type MalojaExportItem struct {
	Time  int64       `json:"time"`
	Track MalojaTrack `json:"track"`
}
type MalojaTrack struct {
	Artists []string `json:"artists"`
	Title   string   `json:"title"`
	Album   struct {
		Title string `json:"albumtitle"`
	} `json:"album"`
}

// MalojaImporter implements the Importer interface for Maloja exports.
type MalojaImporter struct{}

// Name returns the importer source name.
func (m *MalojaImporter) Name() string {
	return "maloja"
}

// ParseRecords parses the Maloja JSON export format.
func (m *MalojaImporter) ParseRecords(data []byte) (interface{}, error) {
	export := new(MalojaExport)
	err := json.Unmarshal(data, &export)
	if err != nil {
		return nil, fmt.Errorf("MalojaImporter.ParseRecords: %w", err)
	}
	return export, nil
}

// ProcessRecords iterates over Maloja scrobbles, validates them, and processes valid entries.
func (m *MalojaImporter) ProcessRecords(ctx context.Context, records interface{}, callback func(opts catalog.SubmitListenOpts) error) (int, error) {
	l := logger.FromContext(ctx)
	export, ok := records.(*MalojaExport)
	if !ok {
		return 0, fmt.Errorf("MalojaImporter.ProcessRecords: invalid records type")
	}

	count := 0
	for _, item := range export.Scrobbles {
		// Skip invalid scrobbles
		if len(item.Track.Artists) < 1 || item.Track.Title == "" {
			l.Debug().Msg("Skipping invalid maloja import item")
			continue
		}

		// Parse timestamp
		ts := time.Unix(item.Time, 0)

		// Check import time window
		if !inImportTimeWindow(ts) {
			l.Debug().Msgf("Skipping import due to import time rules")
			continue
		}

		// Process artist names: Maloja sometimes has artist arrays like ['feature', 'main • feature'],
		// so we normalize and deduplicate them.
		martists := make([]string, 0)
		artists := item.Track.Artists
		artists = utils.MoveFirstMatchToFront(artists, " • ")
		for _, an := range artists {
			ans := strings.Split(an, " • ")
			martists = append(martists, ans...)
		}
		artists = utils.UniqueIgnoringCase(martists)

		opts := catalog.SubmitListenOpts{
			MbzCaller:      &mbz.MusicBrainzClient{},
			Artist:         item.Track.Artists[0],
			ArtistNames:    artists,
			TrackTitle:     item.Track.Title,
			ReleaseTitle:   item.Track.Album.Title,
			Time:           ts.Local(),
			Client:         "maloja",
			UserID:         1,
			SkipCacheImage: !cfg.FetchImagesDuringImport(),
		}

		err := callback(opts)
		if err != nil {
			return count, fmt.Errorf("MalojaImporter.ProcessRecords: %w", err)
		}
		count++
	}

	return count, nil
}

// ImportMalojaFile imports a Maloja export file using the template workflow.
func ImportMalojaFile(ctx context.Context, store db.DB, filename string) error {
	return ImportWithTemplate(ctx, store, filename, &MalojaImporter{})
}
