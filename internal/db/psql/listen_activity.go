package psql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/repository"
)

func validateListenActivityOpts(opts db.ListenActivityOpts) error {
	if opts.Month != 0 && opts.Year == 0 {
		return errors.New("year must be specified with month")
	}
	return nil
}

func normalizeListenActivityOpts(opts db.ListenActivityOpts) db.ListenActivityOpts {
	if opts.Range == 0 {
		opts.Range = db.DefaultRange
	}
	return opts
}

func (d *Psql) logListenActivityFetch(ctx context.Context, opts db.ListenActivityOpts, t1, t2 time.Time, scope string, id int32) {
	l := logger.FromContext(ctx)

	if scope == "" {
		l.Debug().Msgf(
			"Fetching listen activity for %d %s(s) from %v to %v",
			opts.Range,
			opts.Step,
			t1.Format("Jan 02, 2006 15:04:05 MST"),
			t2.Format("Jan 02, 2006 15:04:05 MST"),
		)
		return
	}

	l.Debug().Msgf(
		"Fetching listen activity for %d %s(s) from %v to %v for %s %d",
		opts.Range,
		opts.Step,
		t1.Format("Jan 02, 2006 15:04:05 MST"),
		t2.Format("Jan 02, 2006 15:04:05 MST"),
		scope,
		id,
	)
}

func (d *Psql) GetListenActivity(ctx context.Context, opts db.ListenActivityOpts) ([]db.ListenActivityItem, error) {
	if err := validateListenActivityOpts(opts); err != nil {
		return nil, err
	}

	opts = normalizeListenActivityOpts(opts)
	t1, t2 := db.ListenActivityOptsToTimes(opts)

	switch {
	case opts.AlbumID > 0:
		return d.fetchListenActivityForAlbum(ctx, opts, t1, t2)
	case opts.ArtistID > 0:
		return d.fetchListenActivityForArtist(ctx, opts, t1, t2)
	case opts.TrackID > 0:
		return d.fetchListenActivityForTrack(ctx, opts, t1, t2)
	default:
		return d.fetchAllListenActivity(ctx, opts, t1, t2)
	}
}

func (d *Psql) fetchListenActivityForAlbum(
	ctx context.Context,
	opts db.ListenActivityOpts,
	t1, t2 time.Time,
) ([]db.ListenActivityItem, error) {
	l := logger.FromContext(ctx)
	d.logListenActivityFetch(ctx, opts, t1, t2, "release group", opts.AlbumID)

	rows, err := d.q.ListenActivityForRelease(ctx, repository.ListenActivityForReleaseParams{
		Column1:      opts.Timezone.String(),
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ReleaseID:    opts.AlbumID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetListenActivity: ListenActivityForRelease: %w", err)
	}

	items := make([]db.ListenActivityItem, len(rows))
	for i, row := range rows {
		items[i] = db.ListenActivityItem{
			Start:   row.Day.Time,
			Listens: row.ListenCount,
		}
	}

	l.Debug().Msgf("Database responded with %d steps", len(rows))
	return items, nil
}

func (d *Psql) fetchListenActivityForArtist(
	ctx context.Context,
	opts db.ListenActivityOpts,
	t1, t2 time.Time,
) ([]db.ListenActivityItem, error) {
	l := logger.FromContext(ctx)
	d.logListenActivityFetch(ctx, opts, t1, t2, "artist", opts.ArtistID)

	rows, err := d.q.ListenActivityForArtist(ctx, repository.ListenActivityForArtistParams{
		Column1:      opts.Timezone.String(),
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ArtistID:     opts.ArtistID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetListenActivity: ListenActivityForArtist: %w", err)
	}

	items := make([]db.ListenActivityItem, len(rows))
	for i, row := range rows {
		items[i] = db.ListenActivityItem{
			Start:   row.Day.Time,
			Listens: row.ListenCount,
		}
	}

	l.Debug().Msgf("Database responded with %d steps", len(rows))
	return items, nil
}

func (d *Psql) fetchListenActivityForTrack(
	ctx context.Context,
	opts db.ListenActivityOpts,
	t1, t2 time.Time,
) ([]db.ListenActivityItem, error) {
	l := logger.FromContext(ctx)
	d.logListenActivityFetch(ctx, opts, t1, t2, "track", opts.TrackID)

	rows, err := d.q.ListenActivityForTrack(ctx, repository.ListenActivityForTrackParams{
		Column1:      opts.Timezone.String(),
		ListenedAt:   t1,
		ListenedAt_2: t2,
		ID:           opts.TrackID,
	})
	if err != nil {
		return nil, fmt.Errorf("GetListenActivity: ListenActivityForTrack: %w", err)
	}

	items := make([]db.ListenActivityItem, len(rows))
	for i, row := range rows {
		items[i] = db.ListenActivityItem{
			Start:   row.Day.Time,
			Listens: row.ListenCount,
		}
	}

	l.Debug().Msgf("Database responded with %d steps", len(rows))
	return items, nil
}

func (d *Psql) fetchAllListenActivity(
	ctx context.Context,
	opts db.ListenActivityOpts,
	t1, t2 time.Time,
) ([]db.ListenActivityItem, error) {
	l := logger.FromContext(ctx)
	d.logListenActivityFetch(ctx, opts, t1, t2, "", 0)

	rows, err := d.q.ListenActivity(ctx, repository.ListenActivityParams{
		Column1:      opts.Timezone.String(),
		ListenedAt:   t1,
		ListenedAt_2: t2,
	})
	if err != nil {
		return nil, fmt.Errorf("GetListenActivity: ListenActivity: %w", err)
	}

	items := make([]db.ListenActivityItem, len(rows))
	for i, row := range rows {
		items[i] = db.ListenActivityItem{
			Start:   row.Day.Time,
			Listens: row.ListenCount,
		}
	}

	l.Debug().Msgf("Database responded with %d steps", len(rows))
	return items, nil
}
