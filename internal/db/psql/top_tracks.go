package psql

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/models"
	"github.com/gabehf/koito/internal/repository"
)

func newRankedTrack(
	title string,
	mbzID *uuid.UUID,
	id int32,
	listenCount int64,
	image *uuid.UUID,
	albumID int32,
	artistsJSON []byte,
	rank int64,
) (db.RankedItem[*models.Track], error) {
	artists, err := unmarshalSimpleArtists(artistsJSON)
	if err != nil {
		return db.RankedItem[*models.Track]{}, err
	}

	track := &models.Track{
		Title:       title,
		MbzID:       mbzID,
		ID:          id,
		ListenCount: listenCount,
		Image:       image,
		AlbumID:     albumID,
		Artists:     artists,
	}

	return db.RankedItem[*models.Track]{
		Item: track,
		Rank: rank,
	}, nil
}

func (d *Psql) logTopTracksFetch(ctx context.Context, opts db.GetItemsOpts, t1, t2 time.Time) {
	l := logger.FromContext(ctx)
	l.Debug().Msgf(
		"Fetching top %d tracks on page %d from range %v to %v",
		opts.Limit,
		opts.Page,
		t1.Format("Jan 02, 2006"),
		t2.Format("Jan 02, 2006"),
	)
}

func (d *Psql) GetTopTracksPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Track]], error) {
	offset := pageOffset(opts.Page, opts.Limit)
	opts = normalizeGetItemsOpts(opts)
	t1, t2 := db.TimeframeToTimeRange(opts.Timeframe)

	var (
		tracks []db.RankedItem[*models.Track]
		count  int64
		err    error
	)

	switch {
	case opts.AlbumID > 0:
		tracks, count, err = d.fetchTopTracksByAlbumPage(ctx, opts, t1, t2, offset)
	case opts.ArtistID > 0:
		tracks, count, err = d.fetchTopTracksByArtistPage(ctx, opts, t1, t2, offset)
	default:
		tracks, count, err = d.fetchAllTopTracksPage(ctx, opts, t1, t2, offset)
	}

	if err != nil {
		return nil, err
	}

	return buildPaginatedResponse(tracks, count, opts.Page, opts.Limit, offset), nil
}

func (d *Psql) fetchTopTracksByAlbumPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]db.RankedItem[*models.Track], int64, error) {
	l := logger.FromContext(ctx)
	d.logTopTracksFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetTopTracksInReleasePaginated(ctx, repository.GetTopTracksInReleasePaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
		ReleaseID:    int32(opts.AlbumID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetTopTracksPaginated: GetTopTracksInReleasePaginated: %w", err)
	}

	tracks := make([]db.RankedItem[*models.Track], len(rows))
	for i, row := range rows {
		track, err := newRankedTrack(
			row.Title,
			row.MusicBrainzID,
			row.ID,
			row.ListenCount,
			row.Image,
			row.ReleaseID,
			row.Artists,
			row.Rank,
		)
		if err != nil {
			l.Err(err).Msgf("Error unmarshalling artists for track with id %d", row.ID)
			return nil, 0, fmt.Errorf("GetTopTracksPaginated: Unmarshal: %w", err)
		}
		tracks[i] = track
	}

	count, err := d.q.CountTopTracksByRelease(ctx, repository.CountTopTracksByReleaseParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ReleaseID:    int32(opts.AlbumID),
	})
	if err != nil {
		return nil, 0, err
	}

	return tracks, count, nil
}

func (d *Psql) fetchTopTracksByArtistPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]db.RankedItem[*models.Track], int64, error) {
	l := logger.FromContext(ctx)
	d.logTopTracksFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetTopTracksByArtistPaginated(ctx, repository.GetTopTracksByArtistPaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
		ArtistID:     int32(opts.ArtistID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetTopTracksPaginated: GetTopTracksByArtistPaginated: %w", err)
	}

	tracks := make([]db.RankedItem[*models.Track], len(rows))
	for i, row := range rows {
		track, err := newRankedTrack(
			row.Title,
			row.MusicBrainzID,
			row.ID,
			row.ListenCount,
			row.Image,
			row.ReleaseID,
			row.Artists,
			row.Rank,
		)
		if err != nil {
			l.Err(err).Msgf("Error unmarshalling artists for track with id %d", row.ID)
			return nil, 0, fmt.Errorf("GetTopTracksPaginated: Unmarshal: %w", err)
		}
		tracks[i] = track
	}

	count, err := d.q.CountTopTracksByArtist(ctx, repository.CountTopTracksByArtistParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ArtistID:     int32(opts.ArtistID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetTopTracksPaginated: CountTopTracksByArtist: %w", err)
	}

	return tracks, count, nil
}

func (d *Psql) fetchAllTopTracksPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]db.RankedItem[*models.Track], int64, error) {
	l := logger.FromContext(ctx)
	d.logTopTracksFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetTopTracksPaginated(ctx, repository.GetTopTracksPaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetTopTracksPaginated: GetTopTracksPaginated: %w", err)
	}

	tracks := make([]db.RankedItem[*models.Track], len(rows))
	for i, row := range rows {
		track, err := newRankedTrack(
			row.Title,
			row.MusicBrainzID,
			row.ID,
			row.ListenCount,
			row.Image,
			row.ReleaseID,
			row.Artists,
			row.Rank,
		)
		if err != nil {
			l.Err(err).Msgf("Error unmarshalling artists for track with id %d", row.ID)
			return nil, 0, fmt.Errorf("GetTopTracksPaginated: Unmarshal: %w", err)
		}
		tracks[i] = track
	}

	count, err := d.q.CountTopTracks(ctx, repository.CountTopTracksParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetTopTracksPaginated: CountTopTracks: %w", err)
	}

	l.Debug().Msgf("Database responded with %d tracks out of a total %d", len(rows), count)
	return tracks, count, nil
}
