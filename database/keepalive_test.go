package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/syncromatics/go-kit/v2/database"
	"golang.org/x/sync/errgroup"
	"gotest.tools/assert"
)

var nullResult = sqlmock.NewResult(0, 0)

func Test_SendKeepalivePings_Can_Ping_Continuously(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NilError(t, err)

	// Seems like there should be an easier way to expect this 3 times. Once for
	// the initial query, and then every 100ms for a 250ms total duration.
	mock.ExpectExec("SELECT NULL").WillReturnResult(nullResult)
	mock.ExpectExec("SELECT NULL").WillReturnResult(nullResult)
	mock.ExpectExec("SELECT NULL").WillReturnResult(nullResult)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := 100 * time.Millisecond
	group, ctx := errgroup.WithContext(ctx)

	group.Go(database.SendKeepalivePings(ctx, db, interval))
	select {
	case <-time.After(250 * time.Millisecond):
		cancel()
	case <-ctx.Done():
	}

	err = group.Wait()
	assert.NilError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NilError(t, err)
}

func Test_SendKeepalivePings_Can_Return_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NilError(t, err)

	// Seems like there should be an easier way to expect this 3 times. Once for
	// the initial query, and then every 100ms for a 250ms total duration.
	mock.ExpectExec("SELECT NULL").WillReturnResult(nullResult)
	mock.ExpectExec("SELECT NULL").WillReturnResult(nullResult)
	mock.ExpectExec("SELECT NULL").WillReturnError(fmt.Errorf("broken pipe"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := 100 * time.Millisecond
	group, ctx := errgroup.WithContext(ctx)

	group.Go(database.SendKeepalivePings(ctx, db, interval))
	select {
	case <-time.After(250 * time.Millisecond):
		cancel()
	case <-ctx.Done():
	}

	err = group.Wait()
	assert.ErrorContains(t, err, "broken pipe")

	err = mock.ExpectationsWereMet()
	assert.NilError(t, err)
}
