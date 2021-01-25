package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/pkg/errors"
)

// SendKeepalivePings will periodically send a lightweight query to a database
// in order to keep active, idle connections from being killed by an external
// source.
func SendKeepalivePings(ctx context.Context, db *sql.DB, interval time.Duration) func() error {
	return func() error {
		err := sendPing(ctx, db)
		if err != nil {
			return errors.Wrap(err, "failed sending initial keepalive ping")
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err = sendPing(ctx, db)
				if err != nil {
					return errors.Wrap(err, "failed sending keepalive ping")
				}
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func sendPing(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "SELECT NULL;")
	return err
}
