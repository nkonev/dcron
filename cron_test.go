package dcron

import (
	"context"
	"github.com/robfig/cron/v3"
	"testing"
	"time"

	"github.com/nkonev/dcron/mock_dcron"

	"go.uber.org/mock/gomock"
)

func Test_Cron(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	lock := mock_dcron.NewMockLock(ctrl)

	c := NewCron(WithLock(lock))

	lock.EXPECT().
		Lock(gomock.Any(), gomock.Any(), c.Hostname()).
		Return(true).
		Times(2)

	lock.EXPECT().
		Unlock(gomock.Any(), gomock.Any(), c.Hostname()).
		Times(2)

	job := NewJob("test", "*/5 * * * * *", func(ctx context.Context) error {
		task, _ := TaskFromContext(ctx)
		select {
		case <-ctx.Done():
			t.Logf("exit: %+v", task)
		case <-time.After(time.Second):
			t.Logf("run: %+v", task)
		}
		return nil
	}, WithBeforeFunc(func(task Task) (skip bool) {
		t.Logf("before: %+v", task)
		return false
	}), WithAfterFunc(func(task Task) {
		t.Logf("after: %+v", task)
	}))
	if err := c.AddJobs(job); err != nil {
		t.Fatal(err)
	}
	c.Start()
	c.Run() // should be not working
	time.Sleep(10 * time.Second)
	<-c.Stop().Done()

	t.Logf("cron statistics: %+v", c.Statistics())
	for _, j := range c.Jobs() {
		t.Logf("job %v statistics: %+v", j.Key(), j.Statistics())
	}
}

func TestCron_AddJobs(t *testing.T) {
	c := cron.New(cron.WithSeconds())

	type fields struct {
		hostname string
		cron     *cron.Cron
		lock     Lock
		jobs     []*innerJob
	}
	type args struct {
		jobs []Job
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "regular",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("test_job", "* * * * * *", nil),
				},
			},
			wantErr: false,
		},
		{
			name: "multiple jobs",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("test_job_1", "* * * * * *", nil),
					NewJob("test_job_2", "* * * * * *", nil),
					NewJob("test_job_3", "* * * * * *", nil),
				},
			},
			wantErr: false,
		},
		{
			name: "multiple jobs contain error",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("test_job_1", "* * * * * *", nil),
					NewJob("test_job_2", "* * * * *", nil),
					NewJob("test_job_3", "* * * * * *", nil),
				},
			},
			wantErr: true,
		},
		{
			name: "multiple jobs contain same",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("test_job_1", "* * * * * *", nil),
					NewJob("test_job_2", "* * * * * *", nil),
					NewJob("test_job_1", "* * * * * *", nil),
				},
			},
			wantErr: true,
		},
		{
			name: "with option",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("test_job", "* * * * * *", nil, WithRetryTimes(3)),
				},
			},
			wantErr: false,
		},
		{
			name: "wrong spec",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("test_job", "* * * * *", nil),
				},
			},
			wantErr: true,
		},
		{
			name: "empty key",
			fields: fields{
				cron: c,
			},
			args: args{
				jobs: []Job{
					NewJob("", "* * * * * *", nil),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cron{
				hostname: tt.fields.hostname,
				cron:     tt.fields.cron,
				lock:     tt.fields.lock,
				jobs:     tt.fields.jobs,
			}
			if err := c.AddJobs(tt.args.jobs...); (err != nil) != tt.wantErr {
				t.Errorf("AddJob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCron_Hostname(t *testing.T) {
	type fields struct {
		hostname string
		cron     *cron.Cron
		lock     Lock
		jobs     []*innerJob
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "regular",
			fields: fields{
				hostname: "test_hostname",
			},
			want: "test_hostname",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cron{
				hostname: tt.fields.hostname,
				cron:     tt.fields.cron,
				lock:     tt.fields.lock,
				jobs:     tt.fields.jobs,
			}
			if got := c.Hostname(); got != tt.want {
				t.Errorf("Hostname() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCron(t *testing.T) {
	type args struct {
		options []CronOption
	}
	tests := []struct {
		name  string
		args  args
		check func(t *testing.T, c *Cron)
	}{
		{
			name: "regular",
			args: args{
				options: nil,
			},
			check: func(t *testing.T, c *Cron) {
				if c == nil {
					t.Fatal(t)
				}
			},
		},
		{
			name: "with_option",
			args: args{
				options: []CronOption{},
			},
			check: func(t *testing.T, c *Cron) {
				if c == nil {
					t.Fatal(t)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCron(tt.args.options...)
			tt.check(t, got)
		})
	}
}
