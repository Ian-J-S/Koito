package summary

import (
	"context"
	"fmt"

	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/models"
)

const summaryTopItemsLimit = 5

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

func GenerateSummary(ctx context.Context, store db.DB, opts GenerateSummaryOpts) (*Summary, error) {
	timeframe := opts.Timeframe

	summary := &Summary{
		Title: opts.Title,
	}

	if err := loadTopArtists(ctx, store, summary, timeframe); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	if err := loadTopAlbums(ctx, store, summary, timeframe); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	if err := loadTopTracks(ctx, store, summary, timeframe); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	if err := applyAggregateSummaryStats(ctx, store, summary, timeframe); err != nil {
		return nil, fmt.Errorf("GenerateSummary: %w", err)
	}

	return summary, nil
}

func loadTopArtists(ctx context.Context, store db.DB, summary *Summary, timeframe db.Timeframe) error {
	topArtists, err := store.GetTopArtistsPaginated(ctx, db.GetItemsOpts{
		Page:      1,
		Limit:     summaryTopItemsLimit,
		Timeframe: timeframe,
	})
	if err != nil {
		return err
	}

	summary.TopArtists = topArtists.Items

	return enrichSummaryItems(ctx, store, summary.TopArtists, timeframe, func(artist *models.Artist) db.TimeListenedOpts {
		return db.TimeListenedOpts{ArtistID: artist.ID}
	}, func(artist *models.Artist, timeListened, listens int64) {
		artist.TimeListened = timeListened
		artist.ListenCount = listens
	})
}

func loadTopAlbums(ctx context.Context, store db.DB, summary *Summary, timeframe db.Timeframe) error {
	topAlbums, err := store.GetTopAlbumsPaginated(ctx, db.GetItemsOpts{
		Page:      1,
		Limit:     summaryTopItemsLimit,
		Timeframe: timeframe,
	})
	if err != nil {
		return err
	}

	summary.TopAlbums = topAlbums.Items

	return enrichSummaryItems(ctx, store, summary.TopAlbums, timeframe, func(album *models.Album) db.TimeListenedOpts {
		return db.TimeListenedOpts{AlbumID: album.ID}
	}, func(album *models.Album, timeListened, listens int64) {
		album.TimeListened = timeListened
		album.ListenCount = listens
	})
}

func loadTopTracks(ctx context.Context, store db.DB, summary *Summary, timeframe db.Timeframe) error {
	topTracks, err := store.GetTopTracksPaginated(ctx, db.GetItemsOpts{
		Page:      1,
		Limit:     summaryTopItemsLimit,
		Timeframe: timeframe,
	})
	if err != nil {
		return err
	}

	summary.TopTracks = topTracks.Items

	return enrichSummaryItems(ctx, store, summary.TopTracks, timeframe, func(track *models.Track) db.TimeListenedOpts {
		return db.TimeListenedOpts{TrackID: track.ID}
	}, func(track *models.Track, timeListened, listens int64) {
		track.TimeListened = timeListened
		track.ListenCount = listens
	})
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

func applyAggregateSummaryStats(ctx context.Context, store db.DB, summary *Summary, timeframe db.Timeframe) error {
	daycount := dayCountFromTimeframe(timeframe)

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

	return applySummaryStats(ctx, timeframe, stats)
}

func dayCountFromTimeframe(timeframe db.Timeframe) int {
	t1, t2 := db.TimeframeToTimeRange(timeframe)
	daycount := int(t2.Sub(t1).Hours() / 24)

	if daycount == 0 {
		return 1
	}

	return daycount
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
