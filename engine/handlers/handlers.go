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

const defaultLimitSize = 100
const maximumLimit = 500

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

	// this map is obviously AI.
	// i manually referenced as many links as I could and couldn't find any
	// incorrect entries here so hopefully it is all correct.
	overrides := map[string]string{
		// --- North America ---
		"America/Indianapolis":  "America/Indiana/Indianapolis",
		"America/Knoxville":     "America/Indiana/Knoxville",
		"America/Louisville":    "America/Kentucky/Louisville",
		"America/Montreal":      "America/Toronto",
		"America/Shiprock":      "America/Denver",
		"America/Fort_Wayne":    "America/Indiana/Indianapolis",
		"America/Virgin":        "America/Port_of_Spain",
		"America/Santa_Isabel":  "America/Tijuana",
		"America/Ensenada":      "America/Tijuana",
		"America/Rosario":       "America/Argentina/Cordoba",
		"America/Jujuy":         "America/Argentina/Jujuy",
		"America/Mendoza":       "America/Argentina/Mendoza",
		"America/Catamarca":     "America/Argentina/Catamarca",
		"America/Cordoba":       "America/Argentina/Cordoba",
		"America/Buenos_Aires":  "America/Argentina/Buenos_Aires",
		"America/Coral_Harbour": "America/Atikokan",
		"America/Atka":          "America/Adak",
		"US/Alaska":             "America/Anchorage",
		"US/Aleutian":           "America/Adak",
		"US/Arizona":            "America/Phoenix",
		"US/Central":            "America/Chicago",
		"US/Eastern":            "America/New_York",
		"US/East-Indiana":       "America/Indiana/Indianapolis",
		"US/Hawaii":             "Pacific/Honolulu",
		"US/Indiana-Starke":     "America/Indiana/Knoxville",
		"US/Michigan":           "America/Detroit",
		"US/Mountain":           "America/Denver",
		"US/Pacific":            "America/Los_Angeles",
		"US/Samoa":              "Pacific/Pago_Pago",
		"Canada/Atlantic":       "America/Halifax",
		"Canada/Central":        "America/Winnipeg",
		"Canada/Eastern":        "America/Toronto",
		"Canada/Mountain":       "America/Edmonton",
		"Canada/Newfoundland":   "America/St_Johns",
		"Canada/Pacific":        "America/Vancouver",

		// --- Asia ---
		"Asia/Calcutta":      "Asia/Kolkata",
		"Asia/Saigon":        "Asia/Ho_Chi_Minh",
		"Asia/Katmandu":      "Asia/Kathmandu",
		"Asia/Rangoon":       "Asia/Yangon",
		"Asia/Ulan_Bator":    "Asia/Ulaanbaatar",
		"Asia/Macao":         "Asia/Macau",
		"Asia/Tel_Aviv":      "Asia/Jerusalem",
		"Asia/Ashkhabad":     "Asia/Ashgabat",
		"Asia/Chungking":     "Asia/Chongqing",
		"Asia/Dacca":         "Asia/Dhaka",
		"Asia/Istanbul":      "Europe/Istanbul",
		"Asia/Kashgar":       "Asia/Urumqi",
		"Asia/Thimbu":        "Asia/Thimphu",
		"Asia/Ujung_Pandang": "Asia/Makassar",
		"ROC":                "Asia/Taipei",
		"Iran":               "Asia/Tehran",
		"Israel":             "Asia/Jerusalem",
		"Japan":              "Asia/Tokyo",
		"Singapore":          "Asia/Singapore",
		"Hongkong":           "Asia/Hong_Kong",

		// --- Europe ---
		"Europe/Kiev":     "Europe/Kyiv",
		"Europe/Belfast":  "Europe/London",
		"Europe/Tiraspol": "Europe/Chisinau",
		"Europe/Nicosia":  "Asia/Nicosia",
		"Europe/Moscow":   "Europe/Moscow",
		"W-SU":            "Europe/Moscow",
		"GB":              "Europe/London",
		"GB-Eire":         "Europe/London",
		"Eire":            "Europe/Dublin",
		"Poland":          "Europe/Warsaw",
		"Portugal":        "Europe/Lisbon",
		"Turkey":          "Europe/Istanbul",

		// --- Australia / Pacific ---
		"Australia/ACT":        "Australia/Sydney",
		"Australia/Canberra":   "Australia/Sydney",
		"Australia/LHI":        "Australia/Lord_Howe",
		"Australia/North":      "Australia/Darwin",
		"Australia/NSW":        "Australia/Sydney",
		"Australia/Queensland": "Australia/Brisbane",
		"Australia/South":      "Australia/Adelaide",
		"Australia/Tasmania":   "Australia/Hobart",
		"Australia/Victoria":   "Australia/Melbourne",
		"Australia/West":       "Australia/Perth",
		"Australia/Yancowinna": "Australia/Broken_Hill",
		"Pacific/Samoa":        "Pacific/Pago_Pago",
		"Pacific/Yap":          "Pacific/Chuuk",
		"Pacific/Truk":         "Pacific/Chuuk",
		"Pacific/Ponape":       "Pacific/Pohnpei",
		"NZ":                   "Pacific/Auckland",
		"NZ-CHAT":              "Pacific/Chatham",

		// --- Africa ---
		"Africa/Asmera":   "Africa/Asmara",
		"Africa/Timbuktu": "Africa/Bamako",
		"Egypt":           "Africa/Cairo",
		"Libya":           "Africa/Tripoli",

		// --- Atlantic ---
		"Atlantic/Faeroe":    "Atlantic/Faroe",
		"Atlantic/Jan_Mayen": "Europe/Oslo",
		"Iceland":            "Atlantic/Reykjavik",

		// --- Etc / Misc ---
		"UTC":       "UTC",
		"Etc/UTC":   "UTC",
		"Etc/GMT":   "UTC",
		"GMT":       "UTC",
		"Zulu":      "UTC",
		"Universal": "UTC",
	}

	if cfg.ForceTZ() != nil {
		return cfg.ForceTZ()
	}

	if tz := r.URL.Query().Get("tz"); tz != "" {
		if fixedTz, exists := overrides[tz]; exists {
			tz = fixedTz
		}
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}

	if c, err := r.Cookie("tz"); err == nil {
		var tz string
		if fixedTz, exists := overrides[c.Value]; exists {
			tz = fixedTz
		} else {
			tz = c.Value
		}
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}

	return time.Now().Location()
}
