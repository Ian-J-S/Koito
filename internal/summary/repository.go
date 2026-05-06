package summary

import (
	"context"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/models"
)

type DBWrapper struct {
	db db.DB
}

func NewSummaryRepository(db db.DB) SummaryRepository {
	return &DBWrapper{db: db}
}

func (w *DBWrapper) GetTopArtistsPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Artist]], error) {
	return w.db.GetTopArtistsPaginated(ctx, opts)
}

func (w *DBWrapper) GetTopAlbumsPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Album]], error) {
	return w.db.GetTopAlbumsPaginated(ctx, opts)
}

func (w *DBWrapper) GetTopTracksPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Track]], error) {
	return w.db.GetTopTracksPaginated(ctx, opts)
}

func (w *DBWrapper) CountTimeListenedToItem(ctx context.Context, opts db.TimeListenedOpts) (int64, error) {
	return w.db.CountTimeListenedToItem(ctx, opts)
}

func (w *DBWrapper) CountListensToItem(ctx context.Context, opts db.TimeListenedOpts) (int64, error) {
	return w.db.CountListensToItem(ctx, opts)
}

func (w *DBWrapper) CountTimeListened(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountTimeListened(ctx, timeframe)
}

func (w *DBWrapper) CountListens(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountListens(ctx, timeframe)
}

func (w *DBWrapper) CountTracks(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountTracks(ctx, timeframe)
}

func (w *DBWrapper) CountAlbums(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountAlbums(ctx, timeframe)
}

func (w *DBWrapper) CountArtists(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountArtists(ctx, timeframe)
}

func (w *DBWrapper) CountNewTracks(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountNewTracks(ctx, timeframe)
}

func (w *DBWrapper) CountNewAlbums(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountNewAlbums(ctx, timeframe)
}

func (w *DBWrapper) CountNewArtists(ctx context.Context, timeframe db.Timeframe) (int64, error) {
	return w.db.CountNewArtists(ctx, timeframe)
}