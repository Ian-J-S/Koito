// package handlers implements route handlers
package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
)

const (
	defaultLimitSize = 100
	maximumLimit     = 500
)

var timezoneAliases = map[string]string{
	"America/Knoxville": "America/Indiana/Knox",
}

func parseOptionalInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func parseOptionalInt64(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func OptsFromRequest(r *http.Request) db.GetItemsOpts {
	l := logger.FromContext(r.Context())

	l.Debug().Msg("OptsFromRequest: Parsing query parameters")

	limitStr := r.URL.Query().Get("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		l.Debug().Msgf("OptsFromRequest: Query parameter 'limit' not specified, using default %d", defaultLimitSize)
		limit = defaultLimitSize
	}
	if limit > maximumLimit {
		l.Debug().Msgf("OptsFromRequest: Limit exceeds maximum %d, using default %d", maximumLimit, defaultLimitSize)
		limit = defaultLimitSize
	}

	pageStr := r.URL.Query().Get("page")
	page := 1 // default to 1
	if pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil {
			l.Debug().Msgf("OptsFromRequest: Invalid page parameter '%s', defaulting to 1", pageStr)
			page = 1
		}
	}
	if page < 1 {
		l.Debug().Msg("OptsFromRequest: Page parameter is less than 1, defaulting to 1")
		page = 1
	}

	artistId, err := parseOptionalInt(r.URL.Query().Get("artist_id"))
	if err != nil {
		l.Debug().Msgf("OptsFromRequest: Invalid artist_id parameter '%s', ignoring filter", r.URL.Query().Get("artist_id"))
		artistId = 0
	}

	albumId, err := parseOptionalInt(r.URL.Query().Get("album_id"))
	if err != nil {
		l.Debug().Msgf("OptsFromRequest: Invalid album_id parameter '%s', ignoring filter", r.URL.Query().Get("album_id"))
		albumId = 0
	}

	trackId, err := parseOptionalInt(r.URL.Query().Get("track_id"))
	if err != nil {
		l.Debug().Msgf("OptsFromRequest: Invalid track_id parameter '%s', ignoring filter", r.URL.Query().Get("track_id"))
		trackId = 0
	}

	tf := TimeframeFromRequest(r)

	var period db.Period
	switch strings.ToLower(r.URL.Query().Get("period")) {
	case "day":
		period = db.PeriodDay
	case "week":
		period = db.PeriodWeek
	case "month":
		period = db.PeriodMonth
	case "year":
		period = db.PeriodYear
	case "all_time":
		period = db.PeriodAllTime
	}

	l.Debug().Msgf("OptsFromRequest: Parsed options: limit=%d, page=%d, week=%d, month=%d, year=%d, from=%d, to=%d, artist_id=%d, album_id=%d, track_id=%d, period=%s",
		limit, page, tf.Week, tf.Month, tf.Year, tf.FromUnix, tf.ToUnix, artistId, albumId, trackId, period)

	return db.GetItemsOpts{
		Limit:     limit,
		Page:      page,
		Timeframe: tf,
		ArtistID:  artistId,
		AlbumID:   albumId,
		TrackID:   trackId,
	}
}

func TimeframeFromRequest(r *http.Request) db.Timeframe {
	l := logger.FromContext(r.Context())
	q := r.URL.Query()

	year, err := parseOptionalInt(q.Get("year"))
	if err != nil {
		l.Debug().Msgf("TimeframeFromRequest: Invalid year parameter '%s', ignoring", q.Get("year"))
		year = 0
	}
	month, err := parseOptionalInt(q.Get("month"))
	if err != nil {
		l.Debug().Msgf("TimeframeFromRequest: Invalid month parameter '%s', ignoring", q.Get("month"))
		month = 0
	}
	week, err := parseOptionalInt(q.Get("week"))
	if err != nil {
		l.Debug().Msgf("TimeframeFromRequest: Invalid week parameter '%s', ignoring", q.Get("week"))
		week = 0
	}
	fromUnix, err := parseOptionalInt64(q.Get("from"))
	if err != nil {
		l.Debug().Msgf("TimeframeFromRequest: Invalid from parameter '%s', ignoring", q.Get("from"))
		fromUnix = 0
	}
	toUnix, err := parseOptionalInt64(q.Get("to"))
	if err != nil {
		l.Debug().Msgf("TimeframeFromRequest: Invalid to parameter '%s', ignoring", q.Get("to"))
		toUnix = 0
	}

	return db.Timeframe{
		Period:   db.Period(q.Get("period")),
		Year:     year,
		Month:    month,
		Week:     week,
		FromUnix: fromUnix,
		ToUnix:   toUnix,
		Timezone: parseTZ(r),
	}
}

func parseTZ(r *http.Request) *time.Location {
	if forcedTZ := cfg.ForceTZ(); forcedTZ != nil {
		return forcedTZ
	}

	if loc := loadRequestLocation(r.URL.Query().Get("tz")); loc != nil {
		return loc
	}

	if c, err := r.Cookie("tz"); err == nil {
		if loc := loadRequestLocation(c.Value); loc != nil {
			return loc
		}
	}

	return time.Now().Location()
}

func loadRequestLocation(name string) *time.Location {
	if name == "" {
		return nil
	}

	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}

	if alias, exists := timezoneAliases[name]; exists {
		if loc, err := time.LoadLocation(alias); err == nil {
			return loc
		}
	}

	return nil
}
