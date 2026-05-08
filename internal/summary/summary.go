package summary

import (
	"context"
	"fmt"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/models"
)

const summaryTopItemsLimit = 5
type SummaryRepository interface {
	GetTopArtistsPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Artist]], error)
	GetTopAlbumsPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Album]], error)
	GetTopTracksPaginated(ctx context.Context, opts db.GetItemsOpts) (*db.PaginatedResponse[db.RankedItem[*models.Track]], error)
	CountTimeListenedToItem(ctx context.Context, opts db.TimeListenedOpts) (int64, error)
	CountListensToItem(ctx context.Context, opts db.TimeListenedOpts) (int64, error)
	CountTimeListened(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountListens(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountTracks(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountAlbums(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountArtists(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountNewTracks(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountNewAlbums(ctx context.Context, timeframe db.Timeframe) (int64, error)
	CountNewArtists(ctx context.Context, timeframe db.Timeframe) (int64, error)
}

type Summary struct {
	Title            string                          `json:"title,omitempty"`
	TopArtists       []db.RankedItem[*models.Artist] `json:"top_artists"` // ListenCount and TimeListened are overridden with stats from timeframe
	TopAlbums        []db.RankedItem[*models.Album]  `json:"top_albums"`  // ListenCount and TimeListened are overridden with stats from timeframe
	TopTracks        []db.RankedItem[*models.Track]  `json:"top_tracks"`  // ListenCount and TimeListened are overridden with stats from timeframe
	MinutesListened  int                             `json:"minutes_listened"`
	AvgMinutesPerDay int                             `json:"avg_minutes_listened_per_day"`
	Plays            int                             `json:"plays"`
	AvgPlaysPerDay   float32                         `json:"avg_plays_per_day"`
	UniqueTracks     int                             `json:"unique_tracks"`
	UniqueAlbums     int                             `json:"unique_albums"`
	UniqueArtists    int                             `json:"unique_artists"`
	NewTracks        int                             `json:"new_tracks"`
	NewAlbums        int                             `json:"new_albums"`
	NewArtists       int                             `json:"new_artists"`
}

type GenerateSummaryOpts struct {
	UserID    int32
	Timeframe db.Timeframe
	Title     string
}

func GenerateSummary(ctx context.Context, store db.DB, opts GenerateSummaryOpts) (summary *Summary, err error) {
	userId := opts.UserID
	timeframe := opts.Timeframe
	title := opts.Title

	summary = new(Summary)
	summary.Title = title

	topArtists, err := store.GetTopArtistsPaginated(ctx, db.GetItemsOpts{
		Page:      1,
		Limit:     summaryTopItemsLimit,
		Timeframe: timeframe,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}
	summary.TopArtists = topArtists.Items

	if err = enrichSummaryItems(ctx, store, summary.TopArtists, timeframe, func(artist *models.Artist) db.TimeListenedOpts {
		return db.TimeListenedOpts{ArtistID: artist.ID}
	}, func(artist *models.Artist, timeListened, listens int64) {
		artist.TimeListened = timeListened
		artist.ListenCount = listens
	}); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	topAlbums, err := store.GetTopAlbumsPaginated(ctx, db.GetItemsOpts{
		Page:      1,
		Limit:     summaryTopItemsLimit,
		Timeframe: timeframe,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}
	summary.TopAlbums = topAlbums.Items

	if err = enrichSummaryItems(ctx, store, summary.TopAlbums, timeframe, func(album *models.Album) db.TimeListenedOpts {
		return db.TimeListenedOpts{AlbumID: album.ID}
	}, func(album *models.Album, timeListened, listens int64) {
		album.TimeListened = timeListened
		album.ListenCount = listens
	}); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	topTracks, err := store.GetTopTracksPaginated(ctx, db.GetItemsOpts{
		Page:      1,
		Limit:     summaryTopItemsLimit,
		Timeframe: timeframe,
	})
	if err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}
	summary.TopTracks = topTracks.Items

	if err = enrichSummaryItems(ctx, store, summary.TopTracks, timeframe, func(track *models.Track) db.TimeListenedOpts {
		return db.TimeListenedOpts{TrackID: track.ID}
	}, func(track *models.Track, timeListened, listens int64) {
		track.TimeListened = timeListened
		track.ListenCount = listens
	}); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	t1, t2 := db.TimeframeToTimeRange(timeframe)
	daycount := int(t2.Sub(t1).Hours() / 24)

	// bandaid
	if daycount == 0 {
		daycount = 1
	}

	stats := []summaryStat{
		{
			count: store.CountTimeListened,
			apply: func(count int64) {
				summary.MinutesListened = int(count) / 60
				summary.AvgMinutesPerDay = summary.MinutesListened / daycount
			},
		},
		{
			count: store.CountListens,
			apply: func(count int64) {
				summary.Plays = int(count)
				summary.AvgPlaysPerDay = float32(summary.Plays) / float32(daycount)
			},
		},
		{
			count: store.CountTracks,
			apply: func(count int64) {
				summary.UniqueTracks = int(count)
			},
		},
		{
			count: store.CountAlbums,
			apply: func(count int64) {
				summary.UniqueAlbums = int(count)
			},
		},
		{
			count: store.CountArtists,
			apply: func(count int64) {
				summary.UniqueArtists = int(count)
			},
		},
		{
			count: store.CountNewTracks,
			apply: func(count int64) {
				summary.NewTracks = int(count)
			},
		},
		{
			count: store.CountNewAlbums,
			apply: func(count int64) {
				summary.NewAlbums = int(count)
			},
		},
		{
			count: store.CountNewArtists,
			apply: func(count int64) {
				summary.NewArtists = int(count)
			},
		},
	}

	if err = applySummaryStats(ctx, timeframe, stats); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	return summary, nil
}

func enrichSummaryItems[T any](
	ctx context.Context,
	store db.DB,
	items []db.RankedItem[T],
	timeframe db.Timeframe,
	getOpts func(T) db.TimeListenedOpts,
	setStats func(T, int64, int64),
) error {
	for _, rankedItem := range items {
		countOpts := getOpts(rankedItem.Item)
		countOpts.Timeframe = timeframe

		timeListened, err := store.CountTimeListenedToItem(ctx, countOpts)
		if err != nil {
			return err
		}

		listens, err := store.CountListensToItem(ctx, countOpts)
		if err != nil {
			return err
		}

		setStats(rankedItem.Item, timeListened, listens)
	}

	return nil
}

type summaryStat struct {
	count func(context.Context, db.Timeframe) (int64, error)
	apply func(int64)
}

func applySummaryStats(ctx context.Context, timeframe db.Timeframe, stats []summaryStat) error {
	for _, stat := range stats {
		count, err := stat.count(ctx, timeframe)
		if err != nil {
			return err
		}

		stat.apply(count)
	}

	return nil
}
