package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gabehf/koito/engine/middleware"
	"github.com/gabehf/koito/internal/catalog"
	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/mbz"
	"github.com/gabehf/koito/internal/utils"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"
)

type LbzListenType string

const (
	ListenTypeSingle     LbzListenType = "single"
	ListenTypePlayingNow LbzListenType = "playing_now"
	ListenTypeImport     LbzListenType = "import"
)

type LbzSubmitListenRequest struct {
	ListenType LbzListenType            `json:"listen_type,omitempty"`
	Payload    []LbzSubmitListenPayload `json:"payload,omitempty"`
}

type LbzSubmitListenPayload struct {
	ListenedAt int64        `json:"listened_at,omitempty"`
	TrackMeta  LbzTrackMeta `json:"track_metadata"`
}

type LbzTrackMeta struct {
	ArtistName     string            `json:"artist_name"` // required
	TrackName      string            `json:"track_name"`  // required
	ReleaseName    string            `json:"release_name,omitempty"`
	MBIDMapping    LbzMBIDMapping    `json:"mbid_mapping"`
	AdditionalInfo LbzAdditionalInfo `json:"additional_info,omitempty"`
}

type LbzArtist struct {
	ArtistMBID string `json:"artist_mbid"`
	ArtistName string `json:"artist_credit_name"`
}

type LbzMBIDMapping struct {
	ReleaseMBID   string      `json:"release_mbid"`
	RecordingMBID string      `json:"recording_mbid"`
	ArtistMBIDs   []string    `json:"artist_mbids"`
	Artists       []LbzArtist `json:"artists"`
}

type LbzAdditionalInfo struct {
	MediaPlayer             string   `json:"media_player,omitempty"`
	SubmissionClient        string   `json:"submission_client,omitempty"`
	SubmissionClientVersion string   `json:"submission_client_version,omitempty"`
	ReleaseMBID             string   `json:"release_mbid,omitempty"`
	ReleaseGroupMBID        string   `json:"release_group_mbid,omitempty"`
	ArtistMBIDs             []string `json:"artist_mbids,omitempty"`
	ArtistNames             []string `json:"artist_names,omitempty"`
	RecordingMBID           string   `json:"recording_mbid,omitempty"`
	DurationMs              int32    `json:"duration_ms,omitempty"`
	Duration                int32    `json:"duration,omitempty"`
	Tags                    []string `json:"tags,omitempty"`
	AlbumArtist             string   `json:"albumartist,omitempty"`
}

// Removed Unnecessary parenthesis
const maxListensPerRequest = 1000

var sfGroup singleflight.Group

func LbzSubmitListenHandler(store db.DB, mbzc mbz.MusicBrainzCaller) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logger.FromContext(r.Context())
		l.Debug().Msg("LbzSubmitListenHandler: Received request to submit listens")
		defer func() {
			_ = r.Body.Close()
		}()

		requestBytes, err := readRequestBody(r)
		if err != nil {
			l.Err(err).Msg("LbzSubmitListenHandler: Failed to read request body")
			utils.WriteError(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		if cfg.LbzRelayEnabled() {
			// Fire-and-forget relay; handler result is independent.
			go doLbzRelay(requestBytes, l)
		}

		req, err := decodeSubmitListenRequest(requestBytes)
		if err != nil {
			l.Err(err).Msg("LbzSubmitListenHandler: Failed to decode request")
			utils.WriteError(w, "failed to decode request", http.StatusBadRequest)
			return
		}

		u := middleware.GetUserFromContext(r.Context())
		if u == nil {
			l.Debug().Msg("LbzSubmitListenHandler: Unauthorized request (user context is nil)")
			utils.WriteError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		req.ListenType = normalizeListenType(req.ListenType)

		l.Info().Any("request_body", req).Msg("LbzSubmitListenHandler: Parsed request body")

		if err := validateSubmitListenRequest(req); err != nil {
			l.Debug().AnErr("error", err).Msg("LbzSubmitListenHandler: Request validation failed")
			utils.WriteError(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, payload := range req.Payload {
			if err := validatePayload(payload); err != nil {
				l.Debug().AnErr("error", err).Msg("LbzSubmitListenHandler: Payload validation failed")
				utils.WriteError(w, err.Error(), http.StatusBadRequest)
				return
			}

			opts := buildSubmitListenOpts(r.Context(), store, mbzc, u.ID, req.ListenType, payload, l)

			shared, err := submitListenCoalesced(r.Context(), store, payload, opts)
			if shared {
				l.Info().Msg("LbzSubmitListenHandler: Duplicate requests detected; results were coalesced")
			}
			if err != nil {
				l.Err(err).Msg("LbzSubmitListenHandler: Failed to submit listen")
				writeJSONStatus(w, http.StatusInternalServerError, "internal server error")
				return
			}
		}

		l.Debug().Msg("LbzSubmitListenHandler: Successfully processed listens")
		writeJSONStatus(w, http.StatusOK, "ok")
	}
}

func readRequestBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func decodeSubmitListenRequest(b []byte) (LbzSubmitListenRequest, error) {
	var req LbzSubmitListenRequest
	if err := json.Unmarshal(b, &req); err != nil {
		return LbzSubmitListenRequest{}, err
	}
	return req, nil
}

func normalizeListenType(t LbzListenType) LbzListenType {
	switch t {
	case ListenTypeSingle, ListenTypePlayingNow, ListenTypeImport:
		return t
	case "":
		return ListenTypeSingle
	default:
		// Keep previous behavior: treat invalid types as "single".
		return ListenTypeSingle
	}
}

func validateSubmitListenRequest(req LbzSubmitListenRequest) error {
	if len(req.Payload) < 1 {
		return fmt.Errorf("payload is nil")
	}
	if len(req.Payload) > maxListensPerRequest {
		return fmt.Errorf("payload exceeds max listens per request")
	}

	// For non-import requests, payload must contain exactly one listen.
	if req.ListenType != ListenTypeImport && len(req.Payload) != 1 {
		return fmt.Errorf("payload must only contain one listen for non-import requests")
	}

	return nil
}

func validatePayload(p LbzSubmitListenPayload) error {
	if p.TrackMeta.ArtistName == "" || p.TrackMeta.TrackName == "" {
		return fmt.Errorf("Artist name or track name are missing")
	}
	return nil
}

func buildSubmitListenOpts(
	ctx context.Context,
	store db.DB,
	mbzc mbz.MusicBrainzCaller,
	userID uuid.UUID,
	listenType LbzListenType,
	payload LbzSubmitListenPayload,
	l *zerolog.Logger,
) catalog.SubmitListenOpts {
	artistMbzIDs := parseArtistUUIDs(payload.TrackMeta.AdditionalInfo.ArtistMBIDs, payload.TrackMeta.MBIDMapping.ArtistMBIDs, l)

	rgMbzID := parseUUIDOrNil(payload.TrackMeta.AdditionalInfo.ReleaseGroupMBID)

	releaseMbzID := parseUUIDFromEither(payload.TrackMeta.AdditionalInfo.ReleaseMBID, payload.TrackMeta.MBIDMapping.ReleaseMBID)
	recordingMbzID := parseUUIDFromEither(payload.TrackMeta.AdditionalInfo.RecordingMBID, payload.TrackMeta.MBIDMapping.RecordingMBID)

	client := chooseClient(payload.TrackMeta.AdditionalInfo)
	duration := chooseDuration(payload.TrackMeta.AdditionalInfo)
	listenedAt := chooseListenedAt(payload.ListenedAt, time.Now())

	artistMbidMap := buildArtistMbidMap(payload.TrackMeta.MBIDMapping.Artists, l)

	return catalog.SubmitListenOpts{
		MbzCaller:          mbzc,
		ArtistNames:        payload.TrackMeta.AdditionalInfo.ArtistNames,
		Artist:             payload.TrackMeta.ArtistName,
		ArtistMbzIDs:       artistMbzIDs,
		TrackTitle:         payload.TrackMeta.TrackName,
		RecordingMbzID:     recordingMbzID,
		ReleaseTitle:       payload.TrackMeta.ReleaseName,
		ReleaseMbzID:       releaseMbzID,
		ReleaseGroupMbzID:  rgMbzID,
		ArtistMbidMappings: artistMbidMap,
		Duration:           duration,
		Time:               listenedAt,
		UserID:             userID,
		Client:             client,
		IsNowPlaying:       listenType == ListenTypePlayingNow,
		SkipSaveListen:     listenType == ListenTypePlayingNow,
	}
}

func parseArtistUUIDs(primary []string, fallback []string, l *zerolog.Logger) []uuid.UUID {
	ids, err := utils.ParseUUIDSlice(primary)
	if err != nil {
		l.Debug().AnErr("error", err).Msg("LbzSubmitListenHandler: Failed to parse one or more artist UUIDs (additional_info.artist_mbids)")
	}
	if len(ids) > 0 {
		return ids
	}

	ids, err = utils.ParseUUIDSlice(fallback)
	if err != nil {
		l.Debug().AnErr("error", err).Msg("LbzSubmitListenHandler: Failed to parse one or more artist UUIDs (mbid_mapping.artist_mbids)")
	}
	return ids
}

func parseUUIDOrNil(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func parseUUIDFromEither(primary string, fallback string) uuid.UUID {
	if id := parseUUIDOrNil(primary); id != uuid.Nil {
		return id
	}
	return parseUUIDOrNil(fallback)
}

func chooseClient(info LbzAdditionalInfo) string {
	if info.MediaPlayer != "" {
		return info.MediaPlayer
	}
	if info.SubmissionClient != "" {
		return info.SubmissionClient
	}
	return ""
}

func chooseDuration(info LbzAdditionalInfo) int32 {
	if info.Duration != 0 {
		return info.Duration
	}
	if info.DurationMs != 0 {
		return info.DurationMs / 1000
	}
	return 0
}

func chooseListenedAt(listenedAtUnix int64, now time.Time) time.Time {
	if listenedAtUnix != 0 {
		return time.Unix(listenedAtUnix, 0)
	}
	return now
}

func buildArtistMbidMap(artists []LbzArtist, l *zerolog.Logger) []catalog.ArtistMbidMap {
	var out []catalog.ArtistMbidMap
	for _, a := range artists {
		if a.ArtistMBID == "" || a.ArtistName == "" {
			continue
		}
		mbid, err := uuid.Parse(a.ArtistMBID)
		if err != nil {
			l.Debug().AnErr("error", err).Msgf("LbzSubmitListenHandler: Failed to parse UUID for artist '%s'", a.ArtistName)
			continue
		}
		out = append(out, catalog.ArtistMbidMap{Artist: a.ArtistName, Mbid: mbid})
	}
	return out
}

func submitListenCoalesced(ctx context.Context, store db.DB, payload LbzSubmitListenPayload, opts catalog.SubmitListenOpts) (shared bool, err error) {
	key := buildCaolescingKey(payload)

	_, err, shared = sfGroup.Do(key, func() (interface{}, error) {
		return struct{}{}, catalog.SubmitListen(ctx, store, opts)
	})
	return shared, err
}

func writeJSONStatus(w http.ResponseWriter, statusCode int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(fmt.Sprintf("{\"status\": %q}", status)))
}

func doLbzRelay(requestBytes []byte, l *zerolog.Logger) {
	defer func() {
		if r := recover(); r != nil {
			l.Error().Interface("recover", r).Msg("doLbzRelay: Panic occurred")
		}
	}()

	const (
		maxRetryDuration = 3 * time.Minute
		initialBackoff   = 5 * time.Second
		maxBackoff       = 40 * time.Second
	)

	l.Debug().Msg("doLbzRelay: Building ListenBrainz relay request")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	newReq := func() (*http.Request, error) {
		req, err := http.NewRequest("POST", cfg.LbzRelayUrl()+"/submit-listens", bytes.NewReader(requestBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Token "+cfg.LbzRelayToken())
		req.Header.Add("Content-Type", "application/json")
		return req, nil
	}

	start := time.Now()
	backoff := initialBackoff

	for {
		req, err := newReq()
		if err != nil {
			l.Err(err).Msg("doLbzRelay: Failed to build ListenBrainz relay request")
			return
		}

		l.Debug().Msg("doLbzRelay: Sending ListenBrainz relay request")
		resp, err := client.Do(req)
		if err != nil {
			l.Err(err).Msg("doLbzRelay: Failed to send ListenBrainz relay request")
			return
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			l.Info().Msg("doLbzRelay: Successfully relayed ListenBrainz submission")
			return
		}

		// Retry 5XX responses for up to maxRetryDuration.
		if resp.StatusCode >= 500 && time.Since(start)+backoff <= maxRetryDuration {
			l.Warn().
				Int("status", resp.StatusCode).
				Str("response", string(body)).
				Msg("doLbzRelay: Retryable server error from ListenBrainz relay, retrying...")

			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		l.Warn().
			Int("status", resp.StatusCode).
			Str("response", string(body)).
			Msg("doLbzRelay: Non-2XX response from ListenBrainz relay")
		return
	}
}

func buildCaolescingKey(p LbzSubmitListenPayload) string {
	// the key not including the listen_type introduces the very rare possibility of a playing_now
	// request taking precedence over a single, meaning that a listen will not be logged when it
	// should, however that would require a playing_now request to fire a few seconds before a 'single'
	// of the same track, which should never happen outside of misbehaving clients
	//
	// this could be fixed by restructuring the database inserts for idempotency, which would
	// eliminate the need to coalesce responses, however i'm not gonna do that right now
	return fmt.Sprintf("%s:%s:%s", p.TrackMeta.ArtistName, p.TrackMeta.TrackName, p.TrackMeta.ReleaseName)
}
