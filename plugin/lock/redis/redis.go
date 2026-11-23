package redis

import (
	"context"
	"errors"
	"time"

	redisV9 "github.com/redis/go-redis/v9"

	"github.com/nkonev/dcron"
)

type RedisLock struct {
	client     *redisV9.Client
	logger     dcron.Logger
	slogLogger dcron.SlogLogger
}

func WithLockTTL(duration time.Duration) dcron.JobOption {
	return dcron.WithJobSettings(duration)
}

func WithLock(redisClient *redisV9.Client, options ...RedisLockOption) dcron.CronOption {
	return dcron.WithLock(NewRedisLock(redisClient, options...))
}

const missedDurationMsg = "please provide ttl via WithLockTTL()"

func (m *RedisLock) Validate(jobSettings any, key string) error {
	duration, ok := jobSettings.(time.Duration)
	if !ok {
		return errors.New("unable to cast to time.Duration for task key " + key + ", " + missedDurationMsg)
	}

	if duration <= 0 {
		return errors.New("zero or negative redis ttl for task key " + key)
	}

	return nil
}

func (m *RedisLock) Lock(ctx context.Context, jobSettings any, key, value string) (bool, any, error) {
	duration, ok := jobSettings.(time.Duration)
	if !ok {
		return false, nil, errors.New("unable to cast redis ttl, " + missedDurationMsg)
	}

	locked, err := m.client.SetNX(ctx, key, value, duration).Result()
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("unable to take redis lock %v: %v", key, err)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "unable to take redis lock", dcron.SlogKeyTaskName, key, dcron.SlogKeyError, err)
		}
		return false, nil, err
	}

	return locked, nil, nil
}

func (m *RedisLock) Unlock(ctx context.Context, jobSettings any, key, value string, lockInstance any) error {
	return m.client.Del(ctx, key).Err()
}

func NewRedisLock(redisClient *redisV9.Client, options ...RedisLockOption) *RedisLock {
	ret := &RedisLock{client: redisClient}

	for _, option := range options {
		option(ret)
	}

	return ret
}

type RedisLockOption func(rl *RedisLock)

// WithLog sets the classis logger interface.
func WithLog(logger dcron.Logger) RedisLockOption {
	return func(rl *RedisLock) {
		rl.logger = logger
	}
}

// WithSLog sets the structured logger interface.
func WithSLog(logger dcron.SlogLogger) RedisLockOption {
	return func(rl *RedisLock) {
		rl.slogLogger = logger
	}
}
