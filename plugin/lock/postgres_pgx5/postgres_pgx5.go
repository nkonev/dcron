package redis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nkonev/dcron"
)

type ConnFactory func() (Querier, error)

type PostgresLock struct {
	connFactory ConnFactory

	logger     dcron.Logger
	slogLogger dcron.SlogLogger
}

type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Close(ctx context.Context) error
}

func WithConnString(connString string, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (Querier, error) {
		return pgx.Connect(context.Background(), connString)
	}, options...))
}

func WithConnStringAndOptions(connString string, pgOptions pgx.ParseConfigOptions, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (Querier, error) {
		return pgx.ConnectWithOptions(context.Background(), connString, pgOptions)
	}, options...))
}

func WithConfig(connConfig *pgx.ConnConfig, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (Querier, error) {
		return pgx.ConnectConfig(context.Background(), connConfig)
	}, options...))
}

func WithPool(pool *pgxpool.Pool, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (Querier, error) {
		conn, err := pool.Acquire(context.Background())
		if err != nil {
			return nil, err
		}

		return &poolToQuerierAdapter{
			conn: conn,
		}, nil
	}, options...))
}

type poolToQuerierAdapter struct {
	conn *pgxpool.Conn
}

func (a *poolToQuerierAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return a.conn.QueryRow(ctx, sql, args...)
}

func (a *poolToQuerierAdapter) Close(ctx context.Context) error {
	a.conn.Release()

	return nil
}

const missedKeysMsg = "please provide keys via WithKeys()"

func (m *PostgresLock) Lock(ctx context.Context, jobSettings any, key, value string) (bool, any, error) {
	conn, err := m.connFactory()
	if err != nil {
		return false, nil, fmt.Errorf("unable to instantiate connection: %w", err)
	}

	var success bool

	defer func() {
		if !success {
			cerr := conn.Close(ctx)

			if m.logger != nil {
				m.logger.Errorf("unable to close db in unsuccessful case for %s: %v", key, cerr)
			}
			if m.slogLogger != nil {
				m.slogLogger.ErrorContext(ctx, "unable to close db in unsuccessful case", dcron.SlogKeyTaskName, key, dcron.SlogKeyError, cerr)
			}
		}
	}()

	keys, ok := jobSettings.(argKeys)
	if !ok {
		if m.logger != nil {
			m.logger.Errorf("unable to cast to argKeys %T, "+missedKeysMsg, jobSettings)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "unable to cast to argKeys, "+missedKeysMsg, dcron.SlogKeyTaskName, key)
		}

		return false, nil, errors.New("unable to cast to argKeys, " + missedKeysMsg)
	}

	var locked bool

	r := conn.QueryRow(ctx, "select pg_try_advisory_lock($1, $2)", keys.key1, keys.key2)
	err = r.Scan(&locked)
	if err != nil {
		return false, nil, fmt.Errorf("unable to scan into result: %w", err)
	}

	success = true

	return locked, conn, nil
}

func (m *PostgresLock) Unlock(ctx context.Context, jobSettings any, key, value string, lockValue any) error {
	conn, ok := lockValue.(Querier)
	if !ok {
		return fmt.Errorf("unable to cast lockValue to Querier: got %T", lockValue)
	}
	defer func() {
		cerr := conn.Close(ctx)

		if m.logger != nil {
			m.logger.Errorf("unable to close db in successful case for %s: %v", key, cerr)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "unable to close db in successful case", dcron.SlogKeyTaskName, key, dcron.SlogKeyError, cerr)
		}
	}()

	keys, ok := jobSettings.(argKeys)
	if !ok {
		if m.logger != nil {
			m.logger.Errorf("unable to cast to argKeys %T, "+missedKeysMsg, jobSettings)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "unable to cast to argKeys, "+missedKeysMsg, dcron.SlogKeyTaskName, key)
		}

		return errors.New("unable to cast to argKeys, " + missedKeysMsg)
	}

	var unlocked bool

	r := conn.QueryRow(ctx, "select pg_advisory_unlock($1, $2)", keys.key1, keys.key2)
	err := r.Scan(&unlocked)
	if err != nil {
		return fmt.Errorf("unable to scan into result: %w", err)
	}

	if !unlocked {
		return errors.New("unable to unlock")
	}

	return nil
}

func NewPostgresLock(connFactory ConnFactory, options ...PostgresLockOption) *PostgresLock {
	ret := &PostgresLock{connFactory: connFactory}

	for _, option := range options {
		option(ret)
	}

	return ret
}

type PostgresLockOption func(rl *PostgresLock)

// WithLog sets the classis logger interface.
func WithLog(logger dcron.Logger) PostgresLockOption {
	return func(rl *PostgresLock) {
		rl.logger = logger
	}
}

// WithSLog sets the structured logger interface.
func WithSLog(logger dcron.SlogLogger) PostgresLockOption {
	return func(rl *PostgresLock) {
		rl.slogLogger = logger
	}
}

type argKeys struct {
	key1 int32
	key2 int32
}

// WithKeys sets the keys for advisory lock function.
func WithKeys(key1 int32, key2 int32) dcron.JobOption {
	return dcron.WithJobSettings(argKeys{key1: key1, key2: key2})
}
