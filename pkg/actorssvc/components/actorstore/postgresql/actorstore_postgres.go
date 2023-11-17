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

package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/utils/clock"

	sqlinternal "github.com/dapr/components-contrib/common/component/sql"
	pgmigrations "github.com/dapr/components-contrib/common/component/sql/migrations/postgres"
	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	"github.com/dapr/kit/logger"
)

// Used in tests to allow modifying the connection's configuration object
var modifyConfigFn func(p *PostgreSQL, config *pgxpool.Config)

// NewPostgreSQLActorStore creates a new instance of an actor store backed by PostgreSQL
func NewPostgreSQLActorStore(logger logger.Logger) actorstore.Store {
	return &PostgreSQL{
		logger: logger,
		clock:  clock.RealClock{},
	}
}

type PostgreSQL struct {
	logger   logger.Logger
	metadata pgMetadata
	db       *pgxpool.Pool
	running  atomic.Bool
	clock    clock.Clock
}

func (p *PostgreSQL) Init(ctx context.Context, md actorstore.Metadata) error {
	if !p.running.CompareAndSwap(false, true) {
		return errors.New("already running")
	}

	// Parse metadata
	err := p.metadata.InitWithMetadata(md)
	if err != nil {
		p.logger.Errorf("Failed to parse metadata: %v", err)
		return err
	}

	// Connect to the database
	config, err := p.metadata.GetPgxPoolConfig()
	if err != nil {
		p.logger.Error(err)
		return err
	}

	// Allow modifying the config object when running tests
	if modifyConfigFn != nil {
		modifyConfigFn(p, config)
	}

	connCtx, connCancel := context.WithTimeout(ctx, p.metadata.Timeout)
	p.db, err = pgxpool.NewWithConfig(connCtx, config)
	connCancel()
	if err != nil {
		err = fmt.Errorf("failed to connect to the database: %w", err)
		p.logger.Error(err)
		return err
	}

	err = p.Ping(ctx)
	if err != nil {
		err = fmt.Errorf("failed to ping the database: %w", err)
		p.logger.Error(err)
		return err
	}

	// Migrate schema
	err = p.performMigrations(ctx)
	if err != nil {
		p.logger.Error(err)
		return err
	}

	p.logger.Info("Established connection to PostgreSQL")

	return nil
}

func (p *PostgreSQL) performMigrations(ctx context.Context) error {
	m := pgmigrations.Migrations{
		DB:                p.db,
		Logger:            p.logger,
		MetadataTableName: p.metadata.MetadataTableName,
		MetadataKey:       "migrations-actorstore",
	}

	var (
		hostsTable             = p.metadata.TableName(pgTableHosts)
		hostsActorTypesTable   = p.metadata.TableName(pgTableHostsActorTypes)
		actorsTable            = p.metadata.TableName(pgTableActors)
		remindersTable         = p.metadata.TableName(pgTableReminders)
		fetchRemindersFunction = p.metadata.FunctionName(pgFunctionFetchReminders)
	)

	return m.Perform(ctx, []sqlinternal.MigrationFn{
		// Migration 1: create the tables for actors state
		func(ctx context.Context) error {
			p.logger.Infof("Creating tables for actors state. Hosts table: '%s'. Hosts actor types table: '%s'. Actors table: '%s'", hostsTable, hostsActorTypesTable, actorsTable)
			_, err := p.db.Exec(ctx,
				fmt.Sprintf(migration1Query, hostsTable, hostsActorTypesTable, actorsTable, p.metadata.MetadataTableName, p.metadata.TablePrefix),
			)
			if err != nil {
				return fmt.Errorf("failed to create actors state tables: %w", err)
			}
			return nil
		},
		// Migration 2: create the tables for reminders
		func(ctx context.Context) error {
			p.logger.Infof("Creating tables for reminders. Reminders table: '%s'", remindersTable)
			_, err := p.db.Exec(ctx,
				fmt.Sprintf(migration2Query, remindersTable),
			)
			if err != nil {
				return fmt.Errorf("failed to create reminders table: %w", err)
			}
			return nil
		},
		// Migration 3: create the function for fetching reminders
		func(ctx context.Context) error {
			p.logger.Infof("Creating function for fetching reminders. Function name: '%s'", fetchRemindersFunction)
			_, err := p.db.Exec(ctx,
				fmt.Sprintf(migration3Query, fetchRemindersFunction, remindersTable, hostsTable, hostsActorTypesTable, actorsTable),
			)
			if err != nil {
				return fmt.Errorf("failed to create function for fetching reminders: %w", err)
			}
			return nil
		},
	})
}

func (p *PostgreSQL) Ping(ctx context.Context) error {
	if !p.running.Load() {
		return errors.New("not running")
	}

	ctx, cancel := context.WithTimeout(ctx, p.metadata.Timeout)
	err := p.db.Ping(ctx)
	cancel()
	return err
}

func (p *PostgreSQL) Close() (err error) {
	if !p.running.Load() {
		return nil
	}

	p.logger.Debug("Closing connection")
	if p.db != nil {
		p.db.Close()
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

func executeInTransaction[T any](ctx context.Context, log logger.Logger, db *pgxpool.Pool, timeout time.Duration, fn func(ctx context.Context, tx pgx.Tx) (T, error)) (res T, err error) {
	// Start the transaction
	queryCtx, queryCancel := context.WithTimeout(ctx, timeout)
	defer queryCancel()
	tx, err := db.Begin(queryCtx)
	if err != nil {
		return res, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Rollback in case of failure
	var success bool
	defer func() {
		if success {
			return
		}
		rollbackCtx, rollbackCancel := context.WithTimeout(ctx, timeout)
		defer rollbackCancel()
		rollbackErr := tx.Rollback(rollbackCtx)
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
	queryCtx, queryCancel = context.WithTimeout(ctx, timeout)
	defer queryCancel()
	err = tx.Commit(queryCtx)
	if err != nil {
		return res, fmt.Errorf("failed to commit transaction: %w", err)
	}
	success = true

	return res, nil
}
