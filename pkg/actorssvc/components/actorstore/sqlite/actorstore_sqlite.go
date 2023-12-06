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
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"k8s.io/utils/clock"
	"modernc.org/sqlite"
	sqlitelib "modernc.org/sqlite/lib"

	authSqlite "github.com/dapr/components-contrib/common/authentication/sqlite"
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

	connString, err := s.metadata.GetConnectionString(s.logger, authSqlite.GetConnectionStringOpts{
		EnableForeignKeys: true,
	})
	if err != nil {
		// Already logged
		return err
	}

	s.db, err = sql.Open("sqlite", connString)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	// If the database is in-memory, we can't have more than 1 open connection
	if s.metadata.IsInMemoryDB() {
		s.db.SetMaxOpenConns(1)
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

	// Start watching for updates in background
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
			_, err := m.GetConn().ExecContext(ctx, migration1Query)
			if err != nil {
				return fmt.Errorf("failed to create actors state tables: %w", err)
			}
			return nil
		},
		// Migration 2: create the tables for reminders
		func(ctx context.Context) error {
			s.logger.Info("Creating tables for reminders")
			_, err := m.GetConn().ExecContext(ctx, migration2Query)
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

// Starts watching for updates in background, by periodically polling the metadata table.
// This is meant to be invoked in a background goroutine.
func (s *SQLite) watch() {
	s.logger.Info("Started watching for updates")
	defer s.logger.Info("Stopped watching for updates")

	// Poll every second
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.runningCtx.Done():
			return
		case <-t.C:
			err := s.doWatchUpdates(s.runningCtx)
			if err != nil {
				s.logger.Errorf("Error watching for updates: %v", err)
			}
		}
	}
}

func (s *SQLite) doWatchUpdates(ctx context.Context) error {
	// Select the latest API level
	apiLevel, err := s.getClusterActorsAPILevel(ctx, s.db)
	if err != nil {
		return err
	}

	// Update the API level in the object
	s.updateAPILevel(apiLevel)

	return nil
}

func (s *SQLite) updateAPILevel(apiLevel uint32) {
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

// Retrieves the actors API level for the cluster, from the metadata table.
func (s *SQLite) getClusterActorsAPILevel(ctx context.Context, db querier) (uint32, error) {
	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()

	var (
		val   string
		level uint64
	)
	err := db.QueryRowContext(queryCtx, `SELECT value FROM metadata WHERE key = 'actors-api-level'`).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		// If the row doesn't exist, then assume it's 0
		level = 0
	} else if err != nil {
		return 0, fmt.Errorf("failed to retrieve actors API level from metadata: %w", err)
	} else {
		level, err = strconv.ParseUint(val, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid value in the metadata table for actors API level: %w", err)
		}
		if level > math.MaxUint32 {
			return 0, errors.New("invalid value in the metadata table for actors API level: value larger than MaxUint32")
		}
	}

	// Ok to truncate to 32-bit uint because we already checked the value isn't larger than limit
	return uint32(level), nil
}

// Updates the actors API level for the cluster in the metadata table, returning the updated value.
func (s *SQLite) updateClusterActorsAPILevel(ctx context.Context, db querier) (uint32, error) {
	queryCtx, queryCancel := context.WithTimeout(ctx, s.metadata.Timeout)
	defer queryCancel()
	var val string
	err := db.QueryRowContext(queryCtx, `
WITH c AS (
	SELECT min(host_actors_api_level) AS value
	FROM hosts
)
REPLACE INTO metadata (key, value)
SELECT 'actors-api-level', value FROM c
RETURNING value
`).Scan(&val)
	if err != nil {
		return 0, fmt.Errorf("failed to update actors API level in metadata: %w", err)
	}

	level, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid value in the metadata table for actors API level: %w", err)
	}
	if level > math.MaxUint32 {
		return 0, errors.New("invalid value in the metadata table for actors API level: value larger than MaxUint32")
	}

	// Ok to truncate to 32-bit uint because we already checked the value isn't larger than limit
	return uint32(level), nil
}

// Returns true if the error is a unique constraint violation error, such as a duplicate unique index or primary key.
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && (sqliteErr.Code()&sqlitelib.SQLITE_CONSTRAINT) != 0
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
