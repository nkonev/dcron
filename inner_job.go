package dcron

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
)

// JobMeta is a read only wrapper for innerJob.
type JobMeta interface {
	// Key returns the unique key of the job.
	Key() string
	// Spec returns the spec of the job.
	Spec() string
	// Statistics returns statistics info of the job.
	Statistics() Statistics
}

type innerJob struct {
	cron          *Cron
	entryID       cron.EntryID
	entryGetter   entryGetter
	key           string
	spec          string
	before        BeforeFunc
	run           RunFunc
	after         AfterFunc
	retryTimes    int
	retryInterval RetryInterval
	noLock        bool
	statistics    Statistics
}

// Key implements JobMeta.Key.
func (j *innerJob) Key() string {
	return j.key
}

// Spec implements JobMeta.Spec.
func (j *innerJob) Spec() string {
	return j.spec
}

// Statistics implements JobMeta.Statistics.
func (j *innerJob) Statistics() Statistics {
	return j.statistics
}

func (j *innerJob) Run() {
	c := j.cron
	entry := j.entryGetter.Entry(j.entryID)
	planAt := entry.Prev
	nextAt := entry.Next
	key := j.key

	task := Task{
		Key:        key,
		Cron:       c,
		Job:        j,
		PlanAt:     planAt,
		TriedTimes: 0,
	}
	atomic.AddInt64(&j.statistics.TotalTask, 1)

	parentCtx := c.context
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithDeadline(context.WithValue(parentCtx, keyContextTask, task), nextAt)
	defer cancel()

	if j.before != nil && j.before(task) {
		task.Skipped = true
		atomic.AddInt64(&j.statistics.SkippedTask, 1)
	}

	if !task.Skipped {
		shouldUseLock := func() bool {
			return !j.noLock && j.cron.lock != nil
		}
		shouldExec := func() bool {
			return !shouldUseLock() || j.cron.lock.Lock(ctx, task.Key, c.hostname)
		}
		needExec := shouldExec()

		if needExec {
			beginAt := time.Now()
			task.BeginAt = &beginAt

			for i := 0; i < j.retryTimes; i++ {
				task.Return = safeRun(ctx, j.run)
				atomic.AddInt64(&j.statistics.TotalRun, 1)
				if i > 0 {
					atomic.AddInt64(&j.statistics.RetriedRun, 1)
				}
				task.TriedTimes++
				if task.Return == nil {
					atomic.AddInt64(&j.statistics.PassedRun, 1)
					break
				}
				atomic.AddInt64(&j.statistics.FailedRun, 1)
				if ctx.Err() != nil {
					break
				}
				if j.retryInterval != nil {
					interval := j.retryInterval(task.TriedTimes)
					deadline, _ := ctx.Deadline()
					if -time.Since(deadline) < interval {
						break
					}
					time.Sleep(interval)
				}
			}

			endAt := time.Now()
			task.EndAt = &endAt

			if shouldUseLock() {
				j.cron.lock.Unlock(ctx, task.Key, c.hostname)
			}
		} else {
			task.Missed = true
			atomic.AddInt64(&j.statistics.MissedTask, 1)
		}
	}

	if j.after != nil {
		j.after(task)
	}

	if !task.Skipped && !task.Missed {
		if task.Return == nil {
			atomic.AddInt64(&j.statistics.PassedTask, 1)
		} else {
			atomic.AddInt64(&j.statistics.FailedTask, 1)
		}
	}
}

func safeRun(ctx context.Context, run RunFunc) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v: %s", r, debug.Stack())
		}
	}()
	return run(ctx)
}
