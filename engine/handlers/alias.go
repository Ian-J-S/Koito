package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/models"
	"github.com/gabehf/koito/internal/utils"
)

// entityType represents the type of entity (artist, album, or track).
type entityType int

const (
	entityArtist entityType = iota
	entityAlbum
	entityTrack
)

// parseIDParams extracts and validates ID parameters from query string.
// Returns the entity type, ID value, and an error message if validation fails.
func parseIDParams(artistIDStr, albumIDStr, trackIDStr string) (entityType, int32, string, error) {
	// Check if at least one ID is provided
	if artistIDStr == "" && albumIDStr == "" && trackIDStr == "" {
		return 0, 0, "artist_id, album_id, or track_id must be provided", errors.New("missing id parameters")
	}

	// Check if only one ID is provided
	if utils.MoreThanOneString(artistIDStr, albumIDStr, trackIDStr) {
		return 0, 0, "only one of artist_id, album_id, or track_id can be provided at a time", errors.New("multiple id parameters")
	}

	if artistIDStr != "" {
		id, err := strconv.Atoi(artistIDStr)
		if err != nil {
			return 0, 0, "invalid artist_id", errors.New("invalid artist_id")
		}
		return entityArtist, int32(id), "", nil
	}

	if albumIDStr != "" {
		id, err := strconv.Atoi(albumIDStr)
		if err != nil {
			return 0, 0, "invalid album_id", errors.New("invalid album_id")
		}
		return entityAlbum, int32(id), "", nil
	}

	id, err := strconv.Atoi(trackIDStr)
	if err != nil {
		return 0, 0, "invalid track_id", errors.New("invalid track_id")
	}
	return entityTrack, int32(id), "", nil
}

// fetchAliases retrieves aliases based on entity type and ID.
func fetchAliases(ctx context.Context, store db.DB, et entityType, id int32) ([]models.Alias, error) {
	switch et {
	case entityArtist:
		return store.GetAllArtistAliases(ctx, id)
	case entityAlbum:
		return store.GetAllAlbumAliases(ctx, id)
	case entityTrack:
		return store.GetAllTrackAliases(ctx, id)
	default:
		return nil, errors.New("unknown entity type")
	}
}

const manualAliasSource = "Manual"

func deleteAlias(
	ctx context.Context,
	store db.DB,
	et entityType,
	id int32,
	alias string,
) error {
	switch et {
	case entityArtist:
		return store.DeleteArtistAlias(ctx, id, alias)
	case entityAlbum:
		return store.DeleteAlbumAlias(ctx, id, alias)
	case entityTrack:
		return store.DeleteTrackAlias(ctx, id, alias)
	default:
		return errors.New("unknown entity type")
	}
}

func saveAlias(
	ctx context.Context,
	store db.DB,
	et entityType,
	id int32,
	alias string,
) error {
	aliases := []string{alias}

	switch et {
	case entityArtist:
		return store.SaveArtistAliases(ctx, id, aliases, manualAliasSource)
	case entityAlbum:
		return store.SaveAlbumAliases(ctx, id, aliases, manualAliasSource)
	case entityTrack:
		return store.SaveTrackAliases(ctx, id, aliases, manualAliasSource)
	default:
		return errors.New("unknown entity type")
	}
}

func setPrimaryAlias(
	ctx context.Context,
	store db.DB,
	et entityType,
	id int32,
	alias string,
) error {
	switch et {
	case entityArtist:
		return store.SetPrimaryArtistAlias(ctx, id, alias)
	case entityAlbum:
		return store.SetPrimaryAlbumAlias(ctx, id, alias)
	case entityTrack:
		return store.SetPrimaryTrackAlias(ctx, id, alias)
	default:
		return errors.New("unknown entity type")
	}
}

// GetAliasesHandler retrieves all aliases for a given artist, album, or track ID.
func GetAliasesHandler(store db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		l.Debug().Msgf("GetAliasesHandler: Got request with params: '%s'", r.URL.Query().Encode())

		et, id, msg, err := parseIDParams(
			r.URL.Query().Get("artist_id"),
			r.URL.Query().Get("album_id"),
			r.URL.Query().Get("track_id"),
		)
		if err != nil {
			l.Debug().AnErr("error", err).Msg("GetAliasesHandler: Parameter validation failed")
			utils.WriteError(w, msg, http.StatusBadRequest)
			return
		}

		aliases, err := fetchAliases(ctx, store, et, id)
		if err != nil {
			l.Err(err).Msg("GetAliasesHandler: Failed to retrieve aliases")
			utils.WriteError(w, "failed to retrieve aliases", http.StatusInternalServerError)
			return
		}

		utils.WriteJSON(w, http.StatusOK, aliases)
	}
}

// DeleteAliasHandler deletes an alias for a given artist or album ID.
func DeleteAliasHandler(store db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		l.Debug().Msg("DeleteAliasHandler: Got request")

		err := r.ParseForm()
		if err != nil {
			l.Debug().AnErr("error", err).Msg("DeleteAliasHandler: Failed to parse form")
			utils.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		artistIDStr := r.FormValue("artist_id")
		albumIDStr := r.FormValue("album_id")
		trackIDStr := r.FormValue("track_id")
		alias := r.FormValue("alias")

		if alias == "" || (artistIDStr == "" && albumIDStr == "" && trackIDStr == "") {
			l.Debug().Msg("DeleteAliasHandler: Missing alias or ID parameter")
			utils.WriteError(w, "alias or ID must be provided", http.StatusBadRequest)
			return
		}

		et, id, msg, err := parseIDParams(artistIDStr, albumIDStr, trackIDStr)
		if err != nil {
			l.Debug().AnErr("error", err).Msg("DeleteAliasHandler: Parameter validation failed")
			utils.WriteError(w, msg, http.StatusBadRequest)
			return
		}

		if err := deleteAlias(ctx, store, et, id, alias); err != nil {
			l.Err(err).Msg("DeleteAliasHandler: Failed to delete alias")
			utils.WriteError(w, "failed to delete alias", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// CreateAliasHandler creates new aliases for a given artist, album, or track.
func CreateAliasHandler(store db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		l.Debug().Msg("CreateAliasHandler: Got request")

		err := r.ParseForm()
		if err != nil {
			l.Debug().Msg("CreateAliasHandler: Failed to parse form")
			utils.WriteError(w, "form is invalid", http.StatusBadRequest)
			return
		}

		alias := r.FormValue("alias")
		if alias == "" {
			l.Debug().Msg("CreateAliasHandler: Missing alias parameter")
			utils.WriteError(w, "alias must be provided", http.StatusBadRequest)
			return
		}

		et, id, msg, err := parseIDParams(
			r.FormValue("artist_id"),
			r.FormValue("album_id"),
			r.FormValue("track_id"),
		)
		if err != nil {
			l.Debug().AnErr("error", err).Msg("CreateAliasHandler: Parameter validation failed")
			utils.WriteError(w, msg, http.StatusBadRequest)
			return
		}

		if err = saveAlias(ctx, store, et, id, alias); err != nil {
			l.Error().Err(err).Msg("CreateAliasHandler: Failed to save alias")
			utils.WriteError(w, "failed to save alias", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// sets the primary alias for albums, artists, and tracks
func SetPrimaryAliasHandler(store db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)

		l.Debug().Msg("SetPrimaryAliasHandler: Got request")

		err := r.ParseForm()
		if err != nil {
			l.Debug().Msg("SetPrimaryAliasHandler: Failed to parse form")
			utils.WriteError(w, "form is invalid", http.StatusBadRequest)
			return
		}

		alias := r.FormValue("alias")
		if alias == "" {
			l.Debug().Msg("SetPrimaryAliasHandler: Missing alias parameter")
			utils.WriteError(w, "alias must be provided", http.StatusBadRequest)
			return
		}

		et, id, msg, err := parseIDParams(
			r.FormValue("artist_id"),
			r.FormValue("album_id"),
			r.FormValue("track_id"),
		)
		if err != nil {
			l.Debug().AnErr("error", err).Msg("SetPrimaryAliasHandler: Parameter validation failed")
			utils.WriteError(w, msg, http.StatusBadRequest)
			return
		}

		if err = setPrimaryAlias(ctx, store, et, id, alias); err != nil {
			l.Error().Err(err).Msg("SetPrimaryAliasHandler: Failed to set primary alias")
			utils.WriteError(w, "failed to set primary alias", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
