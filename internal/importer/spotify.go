package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gabehf/koito/internal/catalog"
	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/mbz"
)

type SpotifyExportItem struct {
	Timestamp  time.Time `json:"ts"`
	TrackName  string    `json:"master_metadata_track_name"`
	ArtistName string    `json:"master_metadata_album_artist_name"`
	AlbumName  string    `json:"master_metadata_album_album_name"`
	ReasonEnd  string    `json:"reason_end"`
	MsPlayed   int32     `json:"ms_played"`
}

// SpotifyImporter implements the Importer interface for Spotify exports.
type SpotifyImporter struct{}

// Name returns the importer source name.
func (s *SpotifyImporter) Name() string {
	return "spotify"
}

// ParseRecords parses the Spotify JSON export format.
func (s *SpotifyImporter) ParseRecords(data []byte) (interface{}, error) {
	export := make([]SpotifyExportItem, 0)
	err := json.Unmarshal(data, &export)
	if err != nil {
		return nil, fmt.Errorf("SpotifyImporter.ParseRecords: %w", err)
	}
	return export, nil
}

// ProcessRecords iterates over Spotify export items, validates them, and processes valid entries.
func (s *SpotifyImporter) ProcessRecords(ctx context.Context, records interface{}, callback func(opts catalog.SubmitListenOpts) error) (int, error) {
	l := logger.FromContext(ctx)
	export, ok := records.([]SpotifyExportItem)
	if !ok {
		return 0, fmt.Errorf("SpotifyImporter.ProcessRecords: invalid records type")
	}

	count := 0
	for _, item := range export {
		// Skip items that weren't fully played
		if item.ReasonEnd != "trackdone" {
			continue
		}

		// Check import time window
		if !inImportTimeWindow(item.Timestamp) {
			l.Debug().Msgf("Skipping import due to import time rules")
			continue
		}

		// Skip invalid tracks
		if item.TrackName == "" || item.ArtistName == "" {
			l.Debug().Msg("Skipping non-track item")
			continue
		}

		opts := catalog.SubmitListenOpts{
			MbzCaller:      &mbz.MusicBrainzClient{},
			Artist:         item.ArtistName,
			TrackTitle:     item.TrackName,
			ReleaseTitle:   item.AlbumName,
			Duration:       item.MsPlayed / 1000,
			Time:           item.Timestamp,
			Client:         "spotify",
			UserID:         1,
			SkipCacheImage: !cfg.FetchImagesDuringImport(),
		}

		err := callback(opts)
		if err != nil {
			return count, fmt.Errorf("SpotifyImporter.ProcessRecords: %w", err)
		}
		count++
	}

	return count, nil
}

// ImportSpotifyFile imports a Spotify export file using the template workflow.
func ImportSpotifyFile(ctx context.Context, store db.DB, filename string) error {
	return ImportWithTemplate(ctx, store, filename, &SpotifyImporter{})
}
