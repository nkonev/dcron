package redis

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/nkonev/dcron"
)

type connFactory func() (*pgx.Conn, error)

type PostgresLock struct {
	connFactory connFactory

	logger     dcron.Logger
	slogLogger dcron.SlogLogger

	key1 int32
	key2 int32
}

func WithConnString(connString string, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (*pgx.Conn, error) {
		return pgx.Connect(context.Background(), connString)
	}, options...))
}

func WithConnStringAndOptions(connString string, pgOptions pgx.ParseConfigOptions, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (*pgx.Conn, error) {
		return pgx.ConnectWithOptions(context.Background(), connString, pgOptions)
	}, options...))
}

func WithConfig(connConfig *pgx.ConnConfig, options ...PostgresLockOption) dcron.CronOption {
	return dcron.WithLock(NewPostgresLock(func() (*pgx.Conn, error) {
		return pgx.ConnectConfig(context.Background(), connConfig)
	}, options...))
}

func (m *PostgresLock) Lock(ctx context.Context, jobSettings any, key, value string) (bool, any, error) {
	conn, err := m.connFactory()
	if err != nil {
		return false, nil, fmt.Errorf("unable to instantiate connection: %w", err)
	}

	var success bool

	defer func() {
		if !success {
			conn.Close(ctx)
		}
	}()

	var locked bool

	r := conn.QueryRow(ctx, "select pg_try_advisory_lock($1, $2)", m.key1, m.key2)
	err = r.Scan(&locked)
	if err != nil {
		return false, nil, fmt.Errorf("unable to scan into result: %w", err)
	}

	success = true

	return locked, conn, nil
}

func (m *PostgresLock) Unlock(ctx context.Context, jobSetting any, key, value string, lockValue any) error {
	conn, ok := lockValue.(*pgx.Conn)
	if !ok {
		return fmt.Errorf("unable to cast lockValue to *pgx.Conn: got %T", lockValue)
	}
	defer conn.Close(ctx)

	var unlocked bool

	r := conn.QueryRow(ctx, "select pg_advisory_unlock($1, $2)", m.key1, m.key2)
	err := r.Scan(&unlocked)
	if err != nil {
		return fmt.Errorf("unable to scan into result: %w", err)
	}

	if !unlocked {
		return errors.New("unable to unlock")
	}

	return nil
}

func NewPostgresLock(connFactory connFactory, options ...PostgresLockOption) *PostgresLock {
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

// WithKey1 sets the structured logger interface.
func WithKey1(key int32) PostgresLockOption {
	return func(rl *PostgresLock) {
		rl.key1 = key
	}
}

// WithKey2 sets the structured logger interface.
func WithKey2(key int32) PostgresLockOption {
	return func(rl *PostgresLock) {
		rl.key2 = key
	}
}
