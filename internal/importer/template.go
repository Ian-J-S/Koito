package importer

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/gabehf/koito/internal/catalog"
	"github.com/gabehf/koito/internal/cfg"
	"github.com/gabehf/koito/internal/db"
	"github.com/gabehf/koito/internal/logger"
)

// Importer defines the interface for source-specific import logic.
// Implementations should focus only on parsing and mapping, while the template
// handles the common workflow (file handling, iteration, submission, throttling).
type Importer interface {
	// Name returns the human-readable name of the importer source
	Name() string

	// ParseRecords reads and parses the raw data from the file.
	// It should return a slice of parsed records ready for iteration.
	ParseRecords([]byte) (interface{}, error)

	// ProcessRecords takes the parsed records and yields individual items
	// to be processed. It handles validation, filtering, and mapping to SubmitListenOpts.
	// The callback should return true if the item was successfully processed, false to skip.
	ProcessRecords(ctx context.Context, records interface{}, callback func(opts catalog.SubmitListenOpts) error) (int, error)
}

// ImportWithTemplate provides the common import workflow for all importers.
// It handles:
// - File opening and reading
// - Throttling
// - Error handling
// - Completion tracking
//
// Importers only need to implement record parsing and mapping logic.
func ImportWithTemplate(ctx context.Context, store db.DB, filename string, importer Importer) error {
	l := logger.FromContext(ctx)
	l.Info().Msgf("Beginning %s import on file: %s", importer.Name(), filename)

	// Open and read the file
	file, err := os.Open(path.Join(cfg.ConfigDir(), "import", filename))
	if err != nil {
		l.Err(err).Msgf("Failed to read import file: %s", filename)
		return fmt.Errorf("ImportWithTemplate: %w", err)
	}
	defer file.Close()

	// Read file contents
	fileContent := make([]byte, 0)
	buffer := make([]byte, 4096)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			fileContent = append(fileContent, buffer[:n]...)
		}
		if err != nil {
			if err.Error() != "EOF" {
				return fmt.Errorf("ImportWithTemplate: %w", err)
			}
			break
		}
	}

	// Parse records using importer-specific logic
	records, err := importer.ParseRecords(fileContent)
	if err != nil {
		return fmt.Errorf("ImportWithTemplate: %w", err)
	}

	// Setup throttling
	throttleFunc := func() {}
	if ms := cfg.ThrottleImportMs(); ms > 0 {
		throttleFunc = func() {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
	}

	// Process records
	count, err := importer.ProcessRecords(ctx, records, func(opts catalog.SubmitListenOpts) error {
		err := catalog.SubmitListen(ctx, store, opts)
		if err != nil {
			l.Err(err).Msgf("Failed to import %s playback item", importer.Name())
			return err
		}
		throttleFunc()
		return nil
	})

	if err != nil {
		return fmt.Errorf("ImportWithTemplate: %w", err)
	}

	return finishImport(ctx, filename, count)
}
