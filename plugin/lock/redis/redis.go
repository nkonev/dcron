package redis

import (
	"context"
	redisV9 "github.com/redis/go-redis/v9"
	"time"

	"github.com/nkonev/dcron"
)

type RedisLock struct {
	client     *redisV9.Client
	logger     dcron.Logger
	slogLogger dcron.SlogLogger
}

type RedisLockJobSettings struct {
	duration time.Duration
}

func WithLockTTL(duration time.Duration) dcron.JobOption {
	return dcron.WithJobSettings(&RedisLockJobSettings{
		duration: duration,
	})
}

func WithLock(redisClient *redisV9.Client, options ...RedisLockOption) dcron.CronOption {
	return dcron.WithLock(NewRedisLock(redisClient, options...))
}

func (m *RedisLock) Lock(ctx context.Context, jobSettings any, key, value string) bool {
	js, ok := jobSettings.(*RedisLockJobSettings)
	if !ok {
		if m.logger != nil {
			m.logger.Errorf("unable to cast to *RedisLockJobSettings %v", key)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "unable to cast to *RedisLockJobSettings", dcron.SlogKeyTaskName, key)
		}

		return false
	}

	if js.duration == 0 {
		if m.logger != nil {
			m.logger.Errorf("bad zero expiration %v", key)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "bad zero expiration", dcron.SlogKeyTaskName, key)
		}
		return false
	}

	locked, err := m.client.SetNX(ctx, key, value, js.duration).Result()
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("unable to take redis lock %v: %v", key, err)
		}
		if m.slogLogger != nil {
			m.slogLogger.ErrorContext(ctx, "unable to take redis lock", dcron.SlogKeyTaskName, key, dcron.SlogKeyError, err)
		}
		return false
	}

	return locked
}

func (m *RedisLock) Unlock(ctx context.Context, jobSetting any, key, value string) {
	m.client.Del(ctx, key)
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
