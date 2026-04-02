package psql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/models"
	"github.com/gabehf/koito/internal/repository"
)

func normalizeGetItemsOpts(opts db.GetItemsOpts) db.GetItemsOpts {
	if opts.Limit == 0 {
		opts.Limit = DefaultItemsPerPage
	}
	return opts
}

func pageOffset(page, limit int) int {
	return (page - 1) * limit
}

func buildPaginatedResponse[T any](items []T, total int64, page, limit, offset int) *db.PaginatedResponse[T] {
	return &db.PaginatedResponse[T]{
		Items:        items,
		TotalCount:   total,
		ItemsPerPage: int32(limit),
		HasNextPage:  int64(offset+len(items)) < total,
		CurrentPage:  int32(page),
	}
}

func unmarshalSimpleArtists(data []byte) ([]models.SimpleArtist, error) {
	artists := make([]models.SimpleArtist, 0)
	if err := json.Unmarshal(data, &artists); err != nil {
		return nil, err
	}
	return artists, nil
}

func newListen(trackTitle string, trackID int32, listenedAt time.Time, artistsJSON []byte) (*models.Listen, error) {
	artists, err := unmarshalSimpleArtists(artistsJSON)
	if err != nil {
		return nil, err
	}

	return &models.Listen{
		Track: models.Track{
			Title:   trackTitle,
			ID:      trackID,
			Artists: artists,
		},
		Time: listenedAt,
	}, nil
}

func (d *Psql) logListensFetch(ctx context.Context, opts db.GetItemsOpts, t1, t2 time.Time) {
	l := logger.FromContext(ctx)
	l.Debug().Msgf(
		"Fetching %d listens on page %d from range %v to %v",
		opts.Limit,
		opts.Page,
		t1.Format("Jan 02, 2006"),
		t2.Format("Jan 02, 2006"),
	)
}

func (d *Psql) GetListensPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[*models.Listen], error) {
	offset := pageOffset(opts.Page, opts.Limit)
	opts = normalizeGetItemsOpts(opts)
	t1, t2 := db.TimeframeToTimeRange(opts.Timeframe)

	var (
		listens []*models.Listen
		count   int64
		err     error
	)

	switch {
	case opts.TrackID > 0:
		listens, count, err = d.fetchListensFromTrackPage(ctx, opts, t1, t2, offset)
	case opts.AlbumID > 0:
		listens, count, err = d.fetchListensFromAlbumPage(ctx, opts, t1, t2, offset)
	case opts.ArtistID > 0:
		listens, count, err = d.fetchListensFromArtistPage(ctx, opts, t1, t2, offset)
	default:
		listens, count, err = d.fetchAllListensPage(ctx, opts, t1, t2, offset)
	}

	if err != nil {
		return nil, err
	}

	return buildPaginatedResponse(listens, count, opts.Page, opts.Limit, offset), nil
}

func (d *Psql) fetchListensFromTrackPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]*models.Listen, int64, error) {
	d.logListensFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetLastListensFromTrackPaginated(ctx, repository.GetLastListensFromTrackPaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
		ID:           int32(opts.TrackID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: GetLastListensFromTrackPaginated: %w", err)
	}

	listens := make([]*models.Listen, len(rows))
	for i, row := range rows {
		listen, err := newListen(row.TrackTitle, row.TrackID, row.ListenedAt, row.Artists)
		if err != nil {
			return nil, 0, fmt.Errorf("GetListensPaginated: Unmarshal: %w", err)
		}
		listens[i] = listen
	}

	count, err := d.q.CountListensFromTrack(ctx, repository.CountListensFromTrackParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		TrackID:      int32(opts.TrackID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: CountListensFromTrack: %w", err)
	}

	return listens, count, nil
}

func (d *Psql) fetchListensFromAlbumPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]*models.Listen, int64, error) {
	d.logListensFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetLastListensFromReleasePaginated(ctx, repository.GetLastListensFromReleasePaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
		ReleaseID:    int32(opts.AlbumID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: GetLastListensFromReleasePaginated: %w", err)
	}

	listens := make([]*models.Listen, len(rows))
	for i, row := range rows {
		listen, err := newListen(row.TrackTitle, row.TrackID, row.ListenedAt, row.Artists)
		if err != nil {
			return nil, 0, fmt.Errorf("GetListensPaginated: Unmarshal: %w", err)
		}
		listens[i] = listen
	}

	count, err := d.q.CountListensFromRelease(ctx, repository.CountListensFromReleaseParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ReleaseID:    int32(opts.AlbumID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: CountListensFromRelease: %w", err)
	}

	return listens, count, nil
}

func (d *Psql) fetchListensFromArtistPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]*models.Listen, int64, error) {
	d.logListensFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetLastListensFromArtistPaginated(ctx, repository.GetLastListensFromArtistPaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
		ArtistID:     int32(opts.ArtistID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: GetLastListensFromArtistPaginated: %w", err)
	}

	listens := make([]*models.Listen, len(rows))
	for i, row := range rows {
		listen, err := newListen(row.TrackTitle, row.TrackID, row.ListenedAt, row.Artists)
		if err != nil {
			return nil, 0, fmt.Errorf("GetListensPaginated: Unmarshal: %w", err)
		}
		listens[i] = listen
	}

	count, err := d.q.CountListensFromArtist(ctx, repository.CountListensFromArtistParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ArtistID:     int32(opts.ArtistID),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: CountListensFromArtist: %w", err)
	}

	return listens, count, nil
}

func (d *Psql) fetchAllListensPage(
	ctx context.Context,
	opts db.GetItemsOpts,
	t1, t2 time.Time,
	offset int,
) ([]*models.Listen, int64, error) {
	l := logger.FromContext(ctx)
	d.logListensFetch(ctx, opts, t1, t2)

	rows, err := d.q.GetLastListensPaginated(ctx, repository.GetLastListensPaginatedParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
		Limit:        int32(opts.Limit),
		Offset:       int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: GetLastListensPaginated: %w", err)
	}

	listens := make([]*models.Listen, len(rows))
	for i, row := range rows {
		listen, err := newListen(row.TrackTitle, row.TrackID, row.ListenedAt, row.Artists)
		if err != nil {
			return nil, 0, fmt.Errorf("GetListensPaginated: Unmarshal: %w", err)
		}
		listens[i] = listen
	}

	count, err := d.q.CountListens(ctx, repository.CountListensParams{
		ListenedAt:   t1,
		ListenedAt_2: t2,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("GetListensPaginated: CountListens: %w", err)
	}

	l.Debug().Msgf("Database responded with %d tracks out of a total %d", len(rows), count)
	return listens, count, nil
}

func (d *Psql) SaveListen(ctx context.Context, opts db.SaveListenOpts) error {
	l := logger.FromContext(ctx)
	if opts.TrackID == 0 {
		return errors.New("required parameter TrackID missing")
	}
	if opts.Time.IsZero() {
		opts.Time = time.Now()
	}

	var client *string
	if opts.Client != "" {
		client = &opts.Client
	}

	l.Debug().Msgf("Inserting listen for track with id %d at time %v into DB", opts.TrackID, opts.Time)
	return d.q.InsertListen(ctx, repository.InsertListenParams{
		TrackID:    opts.TrackID,
		ListenedAt: opts.Time,
		UserID:     opts.UserID,
		Client:     client,
	})
}

func (d *Psql) DeleteListen(ctx context.Context, trackId int32, listenedAt time.Time) error {
	l := logger.FromContext(ctx)
	if trackId == 0 {
		return errors.New("required parameter 'trackId' missing")
	}

	l.Debug().Msgf("Deleting listen from track %d at time %s from DB", trackId, listenedAt)
	return d.q.DeleteListen(ctx, repository.DeleteListenParams{
		TrackID:    trackId,
		ListenedAt: listenedAt,
	})
}
