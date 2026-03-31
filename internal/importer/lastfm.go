package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
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

// buildSubmitListenOpts constructs the options for submitting a listen
func buildSubmitListenOpts(track LastFMTrack, album string, albumMBID, artistMBID, trackMBID uuid.UUID, ts time.Time, mbzc mbz.MusicBrainzCaller) catalog.SubmitListenOpts {
	artistMbidMap := buildArtistMbidMap(track.Artist.Text, artistMBID)
	return catalog.SubmitListenOpts{
		MbzCaller:          mbzc,
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
}

func ImportLastFMFile(ctx context.Context, store db.DB, mbzc mbz.MusicBrainzCaller, filename string) error {
	l := logger.FromContext(ctx)
	l.Info().Msgf("Beginning LastFM import on file: %s", filename)
	file, err := os.Open(path.Join(cfg.ConfigDir(), "import", filename))
	if err != nil {
		l.Err(err).Msgf("Failed to read import file: %s", filename)
		return fmt.Errorf("ImportLastFMFile: %w", err)
	}
	defer file.Close()
	var throttleFunc = func() {}
	if ms := cfg.ThrottleImportMs(); ms > 0 {
		throttleFunc = func() {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
	}
	export := make([]LastFMExportPage, 0)
	err = json.NewDecoder(file).Decode(&export)
	if err != nil {
		return fmt.Errorf("ImportLastFMFile: %w", err)
	}
	count := 0
	for _, item := range export {
		for _, track := range item.Track {
			if !validateTrack(track) {
				l.Debug().Msg("Skipping invalid LastFM import item")
				continue
			}

			album := normalizeAlbum(track.Album.Text, track.Name)
			albumMBID, artistMBID, trackMBID := parseTrackMBIDs(track)

			ts, ok := parseTrackTimestamp(track, l)
			if !ok {
				continue
			}

			if !inImportTimeWindow(ts) {
				l.Debug().Msgf("Skipping import due to import time rules")
				continue
			}

			opts := buildSubmitListenOpts(track, album, albumMBID, artistMBID, trackMBID, ts, mbzc)
			err = catalog.SubmitListen(ctx, store, opts)
			if err != nil {
				l.Err(err).Msg("Failed to import LastFM playback item")
				return fmt.Errorf("ImportLastFMFile: %w", err)
			}
			count++
			throttleFunc()
		}
	}
	return finishImport(ctx, filename, count)
}
