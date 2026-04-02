package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gabehf/koito/engine/middleware"
	"github.com/gabehf/koito/internal/catalog"
	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/db/psql"
	"github.com/gabehf/koito/internal/images"
	"github.com/gabehf/koito/internal/importer"
	"github.com/gabehf/koito/internal/logger"
	mbz "github.com/gabehf/koito/internal/mbz"
	"github.com/gabehf/koito/internal/models"
	"github.com/gabehf/koito/internal/utils"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

func Run(
	getenv func(string) string,
	w io.Writer,
	version string,
) error {
	if err := cfg.Load(getenv, version); err != nil {
		return fmt.Errorf("Engine: failed to load configuration: %w", err)
	}

	l := logger.Get()
	l.Debug().Msg("Engine: Starting application initialization")
	configureLogging(l, w)

	ctx := logger.NewContext(l)
	l.Info().Msgf("Koito %s", version)

	if err := ensureEngineDirectories(l); err != nil {
		return err
	}

	store, err := connectDatabaseWithRetry(l)
	if err != nil {
		return err
	}
	defer store.Close(ctx)

	logForcedTimezone(l)

	mbzC := newMusicBrainzCaller(l)
	if err := validateSubsonicConfiguration(l); err != nil {
		return err
	}

	initializeImageSources(l)
	if err := ensureDefaultUser(ctx, l, store); err != nil {
		return err
	}

	logHostConfiguration(l)
	logCORSConfiguration(l)
	warnOnInvalidListenBrainzRelayConfig(l)

	httpServer, serverErr, err := startHTTPServer(l, store, mbzC)
	if err != nil {
		return err
	}

	startBackgroundTasks(ctx, l, store, mbzC)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	runErr := waitForShutdown(l, quit, serverErr)
	return shutdown(ctx, l, httpServer, mbzC, runErr)
}

func configureLogging(l *zerolog.Logger, w io.Writer) {
	if cfg.StructuredLogging() {
		l.Debug().Msg("Engine: Enabling structured logging")
		*l = l.Output(w)
		return
	}

	l.Debug().Msg("Engine: Enabling console logging")
	*l = l.Output(zerolog.ConsoleWriter{
		Out:        w,
		TimeFormat: time.RFC3339,
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("\u001b[30;1m>\u001b[0m %s |", i)
		},
	})
}

func ensureEngineDirectories(l *zerolog.Logger) error {
	if err := ensureDirectory(l, cfg.ConfigDir(), true); err != nil {
		return err
	}
	l.Info().Msgf("Engine: Using config directory: %s", cfg.ConfigDir())

	return ensureDirectory(l, path.Join(cfg.ConfigDir(), "import"), false)
}

func ensureDirectory(l *zerolog.Logger, dir string, recursive bool) error {
	kind := "directory"
	if path.Base(dir) == "import" {
		kind = "import directory"
	}

	l.Debug().Msgf("Engine: Checking %s: %s", kind, dir)
	_, err := os.Stat(dir)
	if err == nil {
		return nil
	}

	l.Info().Msgf("Engine: Creating %s: %s", kind, dir)
	if recursive {
		err = os.MkdirAll(dir, 0o744)
	} else {
		err = os.Mkdir(dir, 0o744)
	}
	if err != nil {
		l.Error().Err(err).Msgf("Engine: Failed to create %s", kind)
		return err
	}

	return nil
}

func connectDatabaseWithRetry(l *zerolog.Logger) (*psql.Psql, error) {
	l.Debug().Msg("Engine: Initializing database connection")
	store, err := psql.New()
	for err != nil {
		l.Error().Err(err).Msg("Engine: Failed to connect to database; retrying in 5 seconds")
		time.Sleep(5 * time.Second)
		store, err = psql.New()
	}
	l.Info().Msg("Engine: Database connection established")
	return store, nil
}

func logForcedTimezone(l *zerolog.Logger) {
	if cfg.ForceTZ() != nil {
		l.Debug().Msgf("Engine: Forcing the use of timezone '%s'", cfg.ForceTZ().String())
	}
}

func newMusicBrainzCaller(l *zerolog.Logger) mbz.MusicBrainzCaller {
	l.Debug().Msg("Engine: Initializing MusicBrainz client")
	if !cfg.MusicBrainzDisabled() {
		l.Info().Msg("Engine: MusicBrainz client initialized")
		return mbz.NewMusicBrainzClient()
	}

	l.Warn().Msg("Engine: MusicBrainz client disabled")
	return &mbz.MbzErrorCaller{}
}

func validateSubsonicConfiguration(l *zerolog.Logger) error {
	if !cfg.SubsonicEnabled() {
		return nil
	}

	l.Debug().Msg("Engine: Checking Subsonic configuration")
	pingURL := cfg.SubsonicUrl() + "/rest/ping.view?" + cfg.SubsonicParams() + "&f=json&v=1&c=koito"

	resp, err := http.Get(pingURL)
	if err != nil {
		l.Error().Err(err).Msg("Engine: Failed to contact Subsonic server! Ensure the provided URL is correct")
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Response struct {
			Status string `json:"status"`
		} `json:"subsonic-response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		l.Error().Err(err).Msg("Engine: Failed to parse Subsonic response")
		return err
	}
	if result.Response.Status != "ok" {
		l.Error().Msg("Engine: Provided Subsonic credentials are invalid")
		return fmt.Errorf("Engine: invalid Subsonic credentials")
	}

	l.Info().Msg("Engine: Subsonic credentials validated successfully")
	return nil
}

func initializeImageSources(l *zerolog.Logger) {
	l.Debug().Msg("Engine: Initializing image sources")
	images.Initialize(images.ImageSourceOpts{
		UserAgent:      cfg.UserAgent(),
		EnableCAA:      !cfg.CoverArtArchiveDisabled(),
		EnableDeezer:   !cfg.DeezerDisabled(),
		EnableSubsonic: cfg.SubsonicEnabled(),
		EnableLastFM:   cfg.LastFMApiKey() != "",
	})
	l.Info().Msg("Engine: Image sources initialized")
}

func ensureDefaultUser(ctx context.Context, l *zerolog.Logger, store db.DB) error {
	l.Debug().Msg("Engine: Checking for default user")
	userCount, _ := store.CountUsers(ctx)
	if userCount >= 1 {
		return nil
	}

	l.Info().Msg("Engine: Creating default user")
	user, err := store.SaveUser(ctx, db.SaveUserOpts{
		Username: cfg.DefaultUsername(),
		Password: cfg.DefaultPassword(),
		Role:     models.UserRoleAdmin,
	})
	if err != nil {
		l.Error().Err(err).Msg("Engine: Failed to save default user in database")
		return err
	}

	apiKey, err := utils.GenerateRandomString(48)
	if err != nil {
		l.Error().Err(err).Msg("Engine: Failed to generate default API key")
		return err
	}

	label := "Default"
	_, err = store.SaveApiKey(ctx, db.SaveApiKeyOpts{
		Key:    apiKey,
		UserID: user.ID,
		Label:  label,
	})
	if err != nil {
		l.Error().Err(err).Msg("Engine: Failed to save default API key in database")
		return err
	}

	l.Info().Msgf("Engine: Default user created. Login: %s : %s", cfg.DefaultUsername(), cfg.DefaultPassword())
	return nil
}

func logHostConfiguration(l *zerolog.Logger) {
	l.Debug().Msg("Engine: Checking allowed hosts configuration")
	switch {
	case cfg.AllowAllHosts():
		l.Warn().Msg("Engine: Configuration allows requests from all hosts. This is a potential security risk!")
	case len(cfg.AllowedHosts()) == 0 || cfg.AllowedHosts()[0] == "":
		l.Warn().Msgf("Engine: No hosts allowed! Did you forget to set the %s variable?", cfg.ALLOWED_HOSTS_ENV)
	default:
		l.Info().Msgf("Engine: Allowing hosts: %v", cfg.AllowedHosts())
	}
}

func logCORSConfiguration(l *zerolog.Logger) {
	if len(cfg.AllowedOrigins()) == 0 || cfg.AllowedOrigins()[0] == "" {
		l.Info().Msgf("Engine: Using default CORS policy")
		return
	}

	l.Info().Msgf("Engine: CORS policy: Allowing origins: %v", cfg.AllowedOrigins())
}

func warnOnInvalidListenBrainzRelayConfig(l *zerolog.Logger) {
	if cfg.LbzRelayEnabled() && (cfg.LbzRelayUrl() == "" || cfg.LbzRelayToken() == "") {
		l.Warn().Msg("You have enabled ListenBrainz relay, but either the URL or token is missing. Double check your configuration to make sure it is correct!")
	}
}

func startHTTPServer(l *zerolog.Logger, store db.DB, mbzC mbz.MusicBrainzCaller) (*http.Server, chan error, error) {
	l.Debug().Msg("Engine: Setting up HTTP server")
	var ready atomic.Bool
	mux := chi.NewRouter()
	mux.Use(middleware.WithRequestID)
	mux.Use(middleware.Logger(l))
	mux.Use(chimiddleware.Recoverer)
	mux.Use(chimiddleware.RealIP)
	mux.Use(middleware.AllowedHosts)
	if err := bindRoutes(mux, &ready, store, mbzC); err != nil {
		l.Error().Err(err).Msg("Engine: Failed to bind routes")
		return nil, nil, err
	}

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr(),
		Handler: mux,
	}

	serverErr := make(chan error, 1)
	go func() {
		ready.Store(true)
		l.Info().Msgf("Engine: Listening on %s", cfg.ListenAddr())
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	return httpServer, serverErr, nil
}

func startBackgroundTasks(ctx context.Context, l *zerolog.Logger, store db.DB, mbzC mbz.MusicBrainzCaller) {
	l.Info().Msg("Engine: Beginning startup tasks...")

	l.Debug().Msg("Engine: Checking import configuration")
	if !cfg.SkipImport() {
		go func() {
			RunImporter(l, store, mbzC)
		}()
	}

	l.Info().Msg("Engine: Pruning orphaned images")
	go catalog.PruneOrphanedImages(logger.NewContext(l), store)
	l.Info().Msg("Engine: Running duration backfill task")
	go catalog.BackfillTrackDurationsFromMusicBrainz(ctx, store, mbzC)
	l.Info().Msg("Engine: Attempting to fetch missing artist images")
	go catalog.FetchMissingArtistImages(ctx, store)
	l.Info().Msg("Engine: Attempting to fetch missing album images")
	go catalog.FetchMissingAlbumImages(ctx, store)
	l.Info().Msg("Engine: Initialization finished")
}

func waitForShutdown(l *zerolog.Logger, quit <-chan os.Signal, serverErr <-chan error) error {
	var runErr error
	select {
	case err := <-serverErr:
		l.Error().Err(err).Msg("Engine: HTTP server stopped unexpectedly")
		runErr = err
	case <-quit:
		l.Info().Msg("Engine: Received server shutdown notice")
	}
	return runErr
}

func shutdown(parentCtx context.Context, l *zerolog.Logger, httpServer *http.Server, mbzC mbz.MusicBrainzCaller, runErr error) error {
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()

	l.Info().Msg("Engine: Waiting for all processes to finish")
	mbzC.Shutdown()
	if err := httpServer.Shutdown(ctx); err != nil {
		l.Error().Err(err).Msg("Engine: Error during server shutdown")
		return err
	}
	l.Info().Msg("Engine: Shutdown successful")
	return runErr
}

func RunImporter(l *zerolog.Logger, store db.DB, mbzc mbz.MusicBrainzCaller) {
	l.Debug().Msg("Importer: Checking for import files...")
	files, err := os.ReadDir(path.Join(cfg.ConfigDir(), "import"))
	if err != nil {
		l.Err(err).Msg("Importer: Failed to read files from import dir")
	}
	if len(files) > 0 {
		l.Info().Msg("Importer: Files found in import directory. Attempting to import...")
	} else {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			l.Error().Interface("recover", r).Msg("Importer: Panic when importing files")
		}
	}()
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.Contains(file.Name(), "Streaming_History_Audio") {
			l.Info().Msgf("Importer: Import file %s detecting as being Spotify export", file.Name())
			err := importer.ImportSpotifyFile(logger.NewContext(l), store, file.Name())
			if err != nil {
				l.Err(err).Msgf("Importer: Failed to import file: %s", file.Name())
			}
		} else if strings.Contains(file.Name(), "maloja") {
			l.Info().Msgf("Importer: Import file %s detecting as being Maloja export", file.Name())
			err := importer.ImportMalojaFile(logger.NewContext(l), store, file.Name())
			if err != nil {
				l.Err(err).Msgf("Importer: Failed to import file: %s", file.Name())
			}
		} else if strings.Contains(file.Name(), "recenttracks") {
			l.Info().Msgf("Importer: Import file %s detecting as being ghan.nl LastFM export", file.Name())
			err := importer.ImportLastFMFile(logger.NewContext(l), store, mbzc, file.Name())
			if err != nil {
				l.Err(err).Msgf("Importer: Failed to import file: %s", file.Name())
			}
		} else if strings.Contains(file.Name(), "listenbrainz") {
			l.Info().Msgf("Importer: Import file %s detecting as being ListenBrainz export", file.Name())
			err := importer.ImportListenBrainzExport(logger.NewContext(l), store, mbzc, file.Name())
			if err != nil {
				l.Err(err).Msgf("Importer: Failed to import file: %s", file.Name())
			}
		} else if strings.Contains(file.Name(), "koito") {
			l.Info().Msgf("Importer: Import file %s detecting as being Koito export", file.Name())
			err := importer.ImportKoitoFile(logger.NewContext(l), store, file.Name())
			if err != nil {
				l.Err(err).Msgf("Importer: Failed to import file: %s", file.Name())
			}
		} else {
			l.Warn().Msgf("Importer: File %s not recognized as a valid import file; make sure it is valid and named correctly", file.Name())
		}
	}
}
