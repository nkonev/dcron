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
	logger        Logger
	slogLogger    SlogLogger
}

const (
	SlogKeyTaskName    = "dcron_task_name"
	SlogKeyAttempt     = "dcron_task_attempt"
	SlogKeyMaxAttempts = "dcron_task_max_attempts"
	SlogKeyError       = "dcron_task_error"
	SlogKeyDuration    = "dcron_sleep_duration"
)

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
				if j.logger != nil {
					j.logger.Infof("starting task %v: %v / %v", task.Key, (i + 1), j.retryTimes)
				}
				if j.slogLogger != nil {
					j.slogLogger.InfoContext(ctx, "starting task", SlogKeyTaskName, task.Key, SlogKeyAttempt, (i + 1), SlogKeyMaxAttempts, j.retryTimes)
				}

				task.Return = safeRun(ctx, j.run)
				atomic.AddInt64(&j.statistics.TotalRun, 1)
				if i > 0 {
					atomic.AddInt64(&j.statistics.RetriedRun, 1)
				}
				task.TriedTimes++
				if task.Return == nil {
					atomic.AddInt64(&j.statistics.PassedRun, 1)
					if j.logger != nil {
						j.logger.Infof("task %v was finished successfully", task.Key)
					}
					if j.slogLogger != nil {
						j.slogLogger.InfoContext(ctx, "task was finished successfully", SlogKeyTaskName, task.Key)
					}

					break
				} else {
					if j.logger != nil {
						j.logger.Errorf("an error occurred during task %v execution: %v", task.Key, task.Return)
					}
					if j.slogLogger != nil {
						j.slogLogger.ErrorContext(ctx, "an error occurred during task execution", SlogKeyTaskName, task.Key, SlogKeyError, task.Return)
					}
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
					if j.logger != nil {
						j.logger.Infof("sleeping % for task %v before retry", interval, task.Key)
					}
					if j.slogLogger != nil {
						j.slogLogger.InfoContext(ctx, "sleeping before retry", SlogKeyTaskName, task.Key, SlogKeyDuration, interval)
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

			if j.logger != nil {
				j.logger.Infof("task %v was missed because of lock", task.Key)
			}
			if j.slogLogger != nil {
				j.slogLogger.InfoContext(ctx, "task was missed because of lock", SlogKeyTaskName, task.Key)
			}
		}
	} else {
		if j.logger != nil {
			j.logger.Infof("task %v was skipped by beforeFunc", task.Key)
		}
		if j.slogLogger != nil {
			j.slogLogger.InfoContext(ctx, "task was skipped by beforeFunc", SlogKeyTaskName, task.Key)
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
