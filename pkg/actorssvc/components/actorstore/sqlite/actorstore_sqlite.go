/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	// Blank import for the underlying SQLite Driver.
	_ "modernc.org/sqlite"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"k8s.io/utils/clock"

	sqlinternal "github.com/dapr/components-contrib/common/component/sql"
	sqlitemigrations "github.com/dapr/components-contrib/common/component/sql/migrations/sqlite"
	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	"github.com/dapr/kit/logger"
)

// Interface for both sql.DB and sql.Tx
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// NewSQLiteActorStore creates a new instance of an actor store backed by SQLite
func NewSQLiteActorStore(logger logger.Logger) actorstore.Store {
	return &SQLite{
		logger: logger,
		clock:  clock.RealClock{},
	}
}

type SQLite struct {
	logger   logger.Logger
	metadata sqliteMetadata
	db       *sql.DB
	running  atomic.Bool
	clock    clock.Clock
	apiLevel atomic.Uint32

	runningCtx    context.Context
	runningCancel context.CancelFunc

	onActorsAPILevelUpdate func(apiLevel uint32)
}

func (s *SQLite) Init(ctx context.Context, md actorstore.Metadata) error {
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("already running")
	}

	s.runningCtx, s.runningCancel = context.WithCancel(context.Background())

	// Parse metadata
	err := s.metadata.InitWithMetadata(md)
	if err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	connString, err := s.metadata.GetConnectionString(s.logger)
	if err != nil {
		// Already logged
		return err
	}

	s.db, err = sql.Open("sqlite", connString)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	err = s.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping the database: %w", err)
	}

	// Migrate schema
	err = s.performMigrations(ctx)
	if err != nil {
		s.logger.Error(err)
		return err
	}

	s.logger.Info("Established connection to SQLite")

	// Start watching for notifications in background
	go s.watch()

	return nil
}

func (s *SQLite) performMigrations(ctx context.Context) error {
	m := sqlitemigrations.Migrations{
		Pool:              s.db,
		Logger:            s.logger,
		MetadataTableName: "metadata",
		MetadataKey:       "migrations",
	}

	return m.Perform(ctx, []sqlinternal.MigrationFn{
		// Migration 1: create the tables for actors state
		func(ctx context.Context) error {
			s.logger.Info("Creating tables for actors state")
			_, err := s.db.ExecContext(ctx, migration1Query)
			if err != nil {
				return fmt.Errorf("failed to create actors state tables: %w", err)
			}
			return nil
		},
		// Migration 2: create the tables for reminders
		func(ctx context.Context) error {
			s.logger.Info("Creating tables for reminders")
			_, err := s.db.ExecContext(ctx, migration2Query)
			if err != nil {
				return fmt.Errorf("failed to create reminders table: %w", err)
			}
			return nil
		},
	})
}

func (s *SQLite) SetOnActorsAPILevelUpdate(fn func(apiLevel uint32)) {
	s.onActorsAPILevelUpdate = fn
	if fn != nil {
		fn(s.apiLevel.Load())
	}
}

// Starts watching for notifications in background, by periodically polling the metadata table.
// This is meant to be invoked in a background goroutine.
func (s *SQLite) watch() {
	s.logger.Info("Started watching for notifications")
loop:
	for {
		select {
		case <-s.runningCtx.Done():
			break loop
		default:
			// No-op
		}

		err := s.doListen(s.runningCtx)
		switch {
		case errors.Is(err, context.Canceled):
			break loop
		case err != nil:
			s.logger.Errorf("Error from notification listener, will reconnect: %v", err)
		default:
			// No-op
		}

		// Reconnect after a delay
		select {
		case <-s.runningCtx.Done():
			break loop
		case <-time.After(time.Second):
			// No-op
		}
	}

	s.logger.Info("Stopped listening for notifications")
	return
}

func (s *SQLite) doListen(ctx context.Context) error {
	panic("TODO")
}

//nolint:unused
func (s *SQLite) updatedAPILevel(apiLevel uint32) {
	// If the new value is the same as the old, return
	if s.apiLevel.Swap(apiLevel) == apiLevel {
		return
	}
	if s.onActorsAPILevelUpdate != nil {
		s.onActorsAPILevelUpdate(apiLevel)
	}
	s.logger.Debugf("Updated actors API level in the cluster: %d", apiLevel)
}

func (s *SQLite) Ping(ctx context.Context) error {
	if !s.running.Load() {
		return errors.New("not running")
	}

	ctx, cancel := context.WithTimeout(ctx, s.metadata.Timeout)
	err := s.db.PingContext(ctx)
	cancel()
	return err
}

func (s *SQLite) Close() (err error) {
	if !s.running.Load() {
		return nil
	}

	s.runningCancel()

	s.logger.Debug("Closing connection")
	if s.db != nil {
		s.db.Close()
	}
	return err
}

// Returns true if the error is a unique constraint violation error, such as a duplicate unique index or primary key.
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}

// Returns true if the error indicates that the actor host being added is on an actor API level lower than the current state for the cluster
func isActorHostAPILevelTooLowError(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.RaiseException && strings.Contains(pgErr.Message, "host_actors_api_level")
}

func executeInTransaction[T any](ctx context.Context, log logger.Logger, db *sql.DB, timeout time.Duration, fn func(ctx context.Context, tx *sql.Tx) (T, error)) (res T, err error) {
	// Start the transaction
	// Note that the context here is tied to the entire transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback in case of failure
	var success bool
	defer func() {
		if success {
			return
		}
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			// Log errors only
			log.Errorf("Error while attempting to roll back transaction: %v", rollbackErr)
		}
	}()

	// Execute the callback
	res, err = fn(ctx, tx)
	if err != nil {
		return res, err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return res, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return res, nil
}
