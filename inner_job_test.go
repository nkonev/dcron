package dcron

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/nkonev/dcron/mock_dcron"

	"go.uber.org/mock/gomock"
)

func Test_innerJob_Key(t *testing.T) {
	type fields struct {
		cron          *Cron
		entryID       cron.EntryID
		key           string
		spec          string
		before        BeforeContextFunc
		run           RunFunc
		after         AfterContextFunc
		retryTimes    int
		retryInterval RetryInterval
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "regular",
			fields: fields{
				key: "test_job",
			},
			want: "test_job",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &innerJob{
				cron:          tt.fields.cron,
				entryID:       tt.fields.entryID,
				key:           tt.fields.key,
				spec:          tt.fields.spec,
				ctxBefore:     tt.fields.before,
				run:           tt.fields.run,
				ctxAfter:      tt.fields.after,
				retryTimes:    tt.fields.retryTimes,
				retryInterval: tt.fields.retryInterval,
			}
			if got := j.Key(); got != tt.want {
				t.Errorf("Key() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_innerJob_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEntryGetter := mock_dcron.NewMockentryGetter(ctrl)
	lock := mock_dcron.NewMockLock(ctrl)

	mockEntryGetter.EXPECT().
		Entry(gomock.Any()).
		DoAndReturn(func(id cron.EntryID) cron.Entry {
			now := time.Now()
			return cron.Entry{
				ID:   id,
				Next: now.Add(time.Duration(id) * time.Second),
				Prev: now,
			}
		}).
		MinTimes(1)

	lock.EXPECT().
		Lock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, jobSettings any, key, value string) (bool, any, error) {
			if value == "always_miss" {
				return false, nil, nil
			} else if value == "always_error" {
				return false, nil, errors.New("an error occurred during acquiring lock")
			} else {
				return true, nil, nil
			}
		}).
		MinTimes(1)

	lock.EXPECT().
		Unlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		MinTimes(1)

	type fields struct {
		cron          *Cron
		entryID       cron.EntryID
		entryGetter   entryGetter
		key           string
		spec          string
		deriveContext DeriveContext
		ctxBefore     BeforeContextFunc
		run           RunFunc
		ctxAfter      AfterContextFunc
		retryTimes    int
		retryInterval RetryInterval
	}
	tests := []struct {
		name       string
		fields     fields
		statistics Statistics
	}{
		{
			name: "regular",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if task.Return != nil {
						t.Fatal(task.Return)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  1,
				FailedTask:  0,
				SkippedTask: 0,
				MissedTask:  0,
				TotalRun:    1,
				PassedRun:   1,
				FailedRun:   0,
				RetriedRun:  0,
			},
		},
		{
			name: "skip",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return true
				},
				run: func(ctx context.Context) error {
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if !task.Skipped {
						t.Fatal(task.Skipped)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  0,
				SkippedTask: 1,
				MissedTask:  0,
				TotalRun:    0,
				PassedRun:   0,
				FailedRun:   0,
				RetriedRun:  0,
			},
		},
		{
			name: "retry",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     5,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					return errors.New("show retry")
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if task.Return == nil {
						t.Fatal(task.Return)
					}
					if task.TriedTimes != 10 {
						t.Fatal(task.TriedTimes)
					}
				},
				retryTimes: 10,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  1,
				SkippedTask: 0,
				MissedTask:  0,
				TotalRun:    10,
				PassedRun:   0,
				FailedRun:   10,
				RetriedRun:  9,
			},
		},
		{
			name: "retry with interval",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     5,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					return errors.New("should retry")
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if task.Return == nil {
						t.Fatal(task.Return)
					}
					if task.TriedTimes >= 10 {
						t.Fatal(task.TriedTimes)
					}
				},
				retryTimes: 10,
				retryInterval: func(triedTimes int) time.Duration {
					return time.Duration(triedTimes) * time.Second
				},
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  1,
				SkippedTask: 0,
				MissedTask:  0,
				TotalRun:    3,
				PassedRun:   0,
				FailedRun:   3,
				RetriedRun:  2,
			},
		},
		{
			name: "take too long",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					time.Sleep(2 * time.Second)
					return errors.New("show retry")
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if task.TriedTimes != 1 {
						t.Fatal(task.TriedTimes)
					}
					if task.Return == nil {
						t.Fatal(task.Return)
					}
				},
				retryTimes:    5,
				retryInterval: nil,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  1,
				SkippedTask: 0,
				MissedTask:  0,
				TotalRun:    1,
				PassedRun:   0,
				FailedRun:   1,
				RetriedRun:  0,
			},
		},
		{
			name: "miss",
			fields: fields{
				cron:        NewCron(WithLock(lock), WithHostname("always_miss")),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if !task.Missed {
						t.Fatal(task.Missed)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  0,
				SkippedTask: 0,
				MissedTask:  1,
				TotalRun:    0,
				PassedRun:   0,
				FailedRun:   0,
				RetriedRun:  0,
			},
		},
		{
			name: "lock_failed_miss",
			fields: fields{
				cron:        NewCron(WithLock(lock), WithHostname("always_error")),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if !task.Missed {
						t.Fatal(task.Missed)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  0,
				SkippedTask: 0,
				MissedTask:  1,
				TotalRun:    0,
				PassedRun:   0,
				FailedRun:   0,
				RetriedRun:  0,
				FailedLock:  1,
			},
		},
		{
			name: "panic by calling",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				run: func(ctx context.Context) error {
					panic("not happy")
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if !strings.Contains(task.Return.Error(), "not happy") {
						t.Fatal(task.Return)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  1,
				SkippedTask: 0,
				MissedTask:  0,
				TotalRun:    1,
				PassedRun:   0,
				FailedRun:   1,
				RetriedRun:  0,
			},
		},
		{
			name: "ctx_before_skip",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return true
				},
				run: func(ctx context.Context) error {
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if !task.Skipped {
						t.Fatal("expected task to be skipped")
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				SkippedTask: 1,
			},
		},
		{
			name: "ctx_before_no_skip",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				ctxBefore: func(ctx context.Context, task Task) (skip bool) {
					return false
				},
				run: func(ctx context.Context) error {
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if task.Return != nil {
						t.Fatal(task.Return)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:  1,
				PassedTask: 1,
				TotalRun:   1,
				PassedRun:  1,
			},
		},
		{
			name: "derive_context",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				deriveContext: func(ctx context.Context, task Task) context.Context {
					return context.WithValue(ctx, ctxKey("test_key"), "test_value")
				},
				run: func(ctx context.Context) error {
					if ctx.Value(ctxKey("test_key")) != "test_value" {
						return errors.New("context value not found")
					}
					return nil
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:  1,
				PassedTask: 1,
				TotalRun:   1,
				PassedRun:  1,
			},
		},
		{
			name: "panic by runtime",
			fields: fields{
				cron:        NewCron(WithLock(lock)),
				entryID:     1,
				entryGetter: mockEntryGetter,
				run: func(ctx context.Context) error {
					if time.Now().Year() > 0 {
						ctx = nil
					}
					ctx.Value("test")
					return nil
				},
				ctxAfter: func(ctx context.Context, task Task) {
					if !strings.Contains(task.Return.Error(), "runtime error: invalid memory address or nil pointer dereference") {
						t.Fatal(task.Return)
					}
				},
				retryTimes: 1,
			},
			statistics: Statistics{
				TotalTask:   1,
				PassedTask:  0,
				FailedTask:  1,
				SkippedTask: 0,
				MissedTask:  0,
				TotalRun:    1,
				PassedRun:   0,
				FailedRun:   1,
				RetriedRun:  0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &innerJob{
				cron:          tt.fields.cron,
				entryID:       tt.fields.entryID,
				entryGetter:   tt.fields.entryGetter,
				key:           tt.fields.key,
				spec:          tt.fields.spec,
				deriveContext: tt.fields.deriveContext,
				ctxBefore:     tt.fields.ctxBefore,
				run:           tt.fields.run,
				ctxAfter:      tt.fields.ctxAfter,
				retryTimes:    tt.fields.retryTimes,
				retryInterval: tt.fields.retryInterval,
			}
			j.Run()
			if got := j.Statistics(); got != tt.statistics {
				t.Errorf("Statistics() = %v, want %v", got, tt.statistics)
			}
		})
	}
}

func Test_innerJob_Spec(t *testing.T) {
	type fields struct {
		cron          *Cron
		entryID       cron.EntryID
		key           string
		spec          string
		ctxBefore     BeforeContextFunc
		run           RunFunc
		ctxAfter      AfterContextFunc
		retryTimes    int
		retryInterval RetryInterval
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "regular",
			fields: fields{
				spec: "* * * * * *",
			},
			want: "* * * * * *",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &innerJob{
				cron:          tt.fields.cron,
				entryID:       tt.fields.entryID,
				key:           tt.fields.key,
				spec:          tt.fields.spec,
				ctxBefore:     tt.fields.ctxBefore,
				run:           tt.fields.run,
				ctxAfter:      tt.fields.ctxAfter,
				retryTimes:    tt.fields.retryTimes,
				retryInterval: tt.fields.retryInterval,
			}
			if got := j.Spec(); got != tt.want {
				t.Errorf("Spec() = %v, want %v", got, tt.want)
			}
		})
	}
}
