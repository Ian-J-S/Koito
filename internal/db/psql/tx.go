package psql

import (
	"context"
	"fmt"

	"github.com/gabehf/koito/internal/logger"
	"github.com/gabehf/koito/internal/repository"
	"github.com/jackc/pgx/v5"
)

func (d *Psql) withTx(ctx context.Context, op string, fn func(*repository.Queries) error) error {
	l := logger.FromContext(ctx)

	tx, err := d.conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		l.Err(err).Msg("Failed to begin transaction")
		return fmt.Errorf("%s: BeginTx: %w", op, err)
	}
	defer tx.Rollback(ctx)

	qtx := d.q.WithTx(tx)

	if err := fn(qtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: Commit: %w", op, err)
	}

	return nil
}

func withTxResult[T any](ctx context.Context, d *Psql, op string, fn func(*repository.Queries) (T, error)) (T, error) {
	l := logger.FromContext(ctx)

	tx, err := d.conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		var zero T
		l.Err(err).Msg("Failed to begin transaction")
		return zero, fmt.Errorf("%s: BeginTx: %w", op, err)
	}
	defer tx.Rollback(ctx)

	qtx := d.q.WithTx(tx)

	ret, err := fn(qtx)
	if err != nil {
		return ret, err
	}

	if err := tx.Commit(ctx); err != nil {
		var zero T
		return zero, fmt.Errorf("%s: Commit: %w", op, err)
	}

	return ret, nil
}
