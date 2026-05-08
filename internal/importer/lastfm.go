package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gabehf/koito/internal/catalog"
	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/mbz"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type LastFMExportPage struct {
	Track []LastFMTrack `json:"track"`
}
type LastFMTrack struct {
	Artist LastFMItem    `json:"artist"`
	Images []LastFMImage `json:"image"`
	MBID   string        `json:"mbid"`
	Album  LastFMItem    `json:"album"`
	Name   string        `json:"name"`
	Date   LastFMDate    `json:"date"`
}
type LastFMItem struct {
	MBID string `json:"mbid"`
	Text string `json:"#text"`
}
type LastFMDate struct {
	Unix string `json:"uts"`
	Text string `json:"#text"`
}
type LastFMImage struct {
	Size string `json:"size"`
	Url  string `json:"#text"`
}

// normalizeAlbum returns the album name, using track name as fallback if album is empty
func normalizeAlbum(album, trackName string) string {
	if album == "" {
		return trackName
	}
	return album
}

// validateTrack checks if a track has the required fields for import
func validateTrack(track LastFMTrack) bool {
	return track.Name != "" && track.Artist.Text != ""
}

// parseTrackMBIDs extracts all three MBID fields from a track, returning uuid.Nil for invalid UUIDs
func parseTrackMBIDs(track LastFMTrack) (albumMBID, artistMBID, trackMBID uuid.UUID) {
	var err error
	albumMBID, err = uuid.Parse(track.Album.MBID)
	if err != nil {
		albumMBID = uuid.Nil
	}
	artistMBID, err = uuid.Parse(track.Artist.MBID)
	if err != nil {
		artistMBID = uuid.Nil
	}
	trackMBID, err = uuid.Parse(track.MBID)
	if err != nil {
		trackMBID = uuid.Nil
	}
	return
}

// parseTrackTimestamp parses the timestamp from a track, trying Unix first then falling back to text format
func parseTrackTimestamp(track LastFMTrack, l *zerolog.Logger) (time.Time, bool) {
	unix, err := strconv.ParseInt(track.Date.Unix, 10, 64)
	if err == nil {
		return time.Unix(unix, 0).UTC(), true
	}

	ts, err := time.Parse("02 Jan 2006, 15:04", track.Date.Text)
	if err != nil {
		l.Err(err).Msg("Could not parse time from listen activity, skipping...")
		return time.Time{}, false
	}
	return ts, true
}

// buildArtistMbidMap creates the artist MBID mappings if a valid ID exists
func buildArtistMbidMap(artistName string, artistMBID uuid.UUID) []catalog.ArtistMbidMap {
	if artistMBID == uuid.Nil {
		return nil
	}
	return []catalog.ArtistMbidMap{{Artist: artistName, Mbid: artistMBID}}
}

// LastFMImporter implements the Importer interface for Last.fm exports.
type LastFMImporter struct {
	mbzc mbz.MusicBrainzCaller
}

// NewLastFMImporter creates a new Last.fm importer with the given MusicBrainz caller.
func NewLastFMImporter(mbzc mbz.MusicBrainzCaller) *LastFMImporter {
	return &LastFMImporter{mbzc: mbzc}
}

// Name returns the importer source name.
func (l *LastFMImporter) Name() string {
	return "lastfm"
}

// ParseRecords parses the Last.fm JSON export format.
func (l *LastFMImporter) ParseRecords(data []byte) (interface{}, error) {
	export := make([]LastFMExportPage, 0)
	err := json.Unmarshal(data, &export)
	if err != nil {
		return nil, fmt.Errorf("LastFMImporter.ParseRecords: %w", err)
	}
	return export, nil
}

// ProcessRecords iterates over Last.fm export pages and tracks, validates them, and processes valid entries.
func (l *LastFMImporter) ProcessRecords(ctx context.Context, records interface{}, callback func(opts catalog.SubmitListenOpts) error) (int, error) {
	logger := logger.FromContext(ctx)
	export, ok := records.([]LastFMExportPage)
	if !ok {
		return 0, fmt.Errorf("LastFMImporter.ProcessRecords: invalid records type")
	}

	count := 0
	for _, page := range export {
		for _, track := range page.Track {
			// Validate track
			if !validateTrack(track) {
				logger.Debug().Msg("Skipping invalid LastFM import item")
				continue
			}

			// Normalize album name
			album := normalizeAlbum(track.Album.Text, track.Name)

			// Parse MBIDs
			albumMBID, artistMBID, trackMBID := parseTrackMBIDs(track)

			// Parse timestamp
			ts, ok := parseTrackTimestamp(track, logger)
			if !ok {
				continue
			}

			// Check import time window
			if !inImportTimeWindow(ts) {
				logger.Debug().Msgf("Skipping import due to import time rules")
				continue
			}

			// Build options and process
			artistMbidMap := buildArtistMbidMap(track.Artist.Text, artistMBID)
			opts := catalog.SubmitListenOpts{
				MbzCaller:          l.mbzc,
				Artist:             track.Artist.Text,
				ArtistNames:        []string{track.Artist.Text},
				ArtistMbzIDs:       []uuid.UUID{artistMBID},
				TrackTitle:         track.Name,
				RecordingMbzID:     trackMBID,
				ReleaseTitle:       album,
				ReleaseMbzID:       albumMBID,
				ArtistMbidMappings: artistMbidMap,
				Client:             "lastfm",
				Time:               ts,
				UserID:             1,
				SkipCacheImage:     !cfg.FetchImagesDuringImport(),
			}

			err := callback(opts)
			if err != nil {
				return count, fmt.Errorf("LastFMImporter.ProcessRecords: %w", err)
			}
			count++
		}
	}

	return count, nil
}

// ImportLastFMFile imports a Last.fm export file using the template workflow.
func ImportLastFMFile(ctx context.Context, store db.DB, mbzc mbz.MusicBrainzCaller, filename string) error {
	return ImportWithTemplate(ctx, store, filename, NewLastFMImporter(mbzc))
}
