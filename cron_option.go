package dcron

import (
	"context"
	"time"
)

// CronOption represents a modification to the default behavior of a Cron.
type CronOption func(c *Cron)

// WithHostname overrides the hostname of the cron instance.
func WithHostname(hostname string) CronOption {
	return func(c *Cron) {
		c.hostname = hostname
	}
}

// WithLock uses the provided Lock.
func WithLock(lock Lock) CronOption {
	return func(c *Cron) {
		c.lock = lock
	}
}

// WithLocation overrides the timezone of the cron instance.
func WithLocation(loc *time.Location) CronOption {
	return func(c *Cron) {
		c.location = loc
	}
}

// WithContext sets the root context of the cron instance.
// It will be used as the parent context of all tasks,
// and when the context is done, the cron will be stopped.
func WithContext(ctx context.Context) CronOption {
	return func(c *Cron) {
		c.context, c.contextCancel = context.WithCancel(ctx)
	}
}

// WithLog sets the logger interface.
func WithLog(logger Logger) CronOption {
	return func(c *Cron) {
		c.logger = logger
	}
}
