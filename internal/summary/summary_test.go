package summary_test

import (
	"testing"

	"github.com/gabehf/koito/internal/cfg"
)

func TestMain(t *testing.M) {
	// dir, err := utils.GenerateRandomString(8)
	// if err != nil {
	// 	panic(err)
	// }
	cfg.Load(func(env string) string {
		switch env {
		case cfg.ENABLE_STRUCTURED_LOGGING_ENV:
			return "true"
		case cfg.LOG_LEVEL_ENV:
			return "debug"
		case cfg.DATABASE_URL_ENV:
			return "postgres://postgres:secret@localhost"
		case cfg.CONFIG_DIR_ENV:
			return "."
		case cfg.DISABLE_DEEZER_ENV, cfg.DISABLE_COVER_ART_ARCHIVE_ENV, cfg.DISABLE_MUSICBRAINZ_ENV, cfg.ENABLE_FULL_IMAGE_CACHE_ENV:
			return "true"
		default:
			return ""
		}
	}, "test")
	t.Run()
}

func TestGenerateSummary(t *testing.T) {

}
