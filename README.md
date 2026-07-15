# dcron

[![Go Reference](https://pkg.go.dev/badge/github.com/nkonev/dcron.svg)](https://pkg.go.dev/github.com/nkonev/dcron)
[![Go Report Card](https://goreportcard.com/badge/github.com/nkonev/dcron)](https://goreportcard.com/report/github.com/nkonev/dcron)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/nkonev/dcron)](https://github.com/nkonev/dcron/blob/master/go.mod)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/nkonev/dcron)](https://github.com/nkonev/dcron/tags)

A distributed cron framework.

## Install

```shell
go get github.com/nkonev/dcron
```

## Quick Start

First, implement a distributed lock operation that only requires support for two methods: `Lock` and `Unlock`.
You can implement it in any way you prefer, such as using Redis `SetNX`.

```go
import "github.com/redis/go-redis/v9"

type RedisLock struct {
	client *redis.Client
}

func (m *RedisLock) Lock(ctx context.Context, key, value string) bool {
	ret := m.client.SetNX(ctx, key, value, time.Hour)
	return ret.Err() == nil && ret.Val()
}

func (m *RedisLock) Unlock(ctx context.Context, key, value string) {
	m.client.Del(ctx, key)
}
```

Now you can create a cron with that:

```go
func main() {
	lock := &RedisLock{
		client: redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		}),
	}
	cron := dcron.NewCron(dcron.WithLock(lock))
}
```

Then, create a job and add it to the cron.

```go
	job1 := dcron.NewJob("Job1", "*/15 * * * * *", func(ctx context.Context) error {
		if task, ok := dcron.TaskFromContext(ctx); ok {
			log.Println("run:", task.Job.Spec(), task.Key)
		}
		// do something
		return nil
	})
	if err := cron.AddJobs(job1); err != nil {
		log.Fatal(err)
	}
```

Finally, start the cron:

```go
	cron.Start()
	log.Println("cron started")
	time.Sleep(time.Minute)
	<-cron.Stop().Done()
```

If you start the program multiple times, you will notice that the cron will run the job once every 15 seconds on only one of the processes.

| process 1                                      | process 2                                     | process 3                                     |
|------------------------------------------------|-----------------------------------------------|-----------------------------------------------|
| 2023/10/13 11:39:45 cron started               | 2023/10/13 11:39:47 cron started              | 2023/10/13 11:39:48 cron started              |
|                                                |                                               | 2023/10/13 11:40:00 run: */15 * * * * * Job1  |
|                                                | 2023/10/13 11:40:15 run: */15 * * * * * Job1  |                                               |
|                                                |                                               | 2023/10/13 11:40:30 run: */15 * * * * * Job1  |
| 2023/10/13 11:40:45 run: */15 * * * * * Job1   |                                               |                                               |

One more thing, since `dcron.WithLock(lock)` is optional, it's also a good idea to use it as a local cron.

```go
	cron := dcron.NewCron()
	job2 := dcron.NewJob("A local job", "*/15 * * * * *", func(ctx context.Context) error {
		// do something
		return nil
	})
	if err := cron.AddJobs(job2); err != nil {
		log.Fatal(err)
	}
```

## Logging

There is support of classis and structured contextual loggers (slog) via thin `dcron.Logger` and `dcron.SlogLogger` interfaces
```go
type Logger interface {
	Errorf(msgf string, args ...any) // args are args into format string `msgf`
	Infof(msgf string, args ...any)
}

type SlogLogger interface {
	ErrorContext(ctx context.Context, msg string, args ...any) // args are structured attributes
	InfoContext(ctx context.Context, msg string, args ...any)
}
```

To set a custom log level for `dcron` you need to write so-called log adapter:
```go
	cron := dcron.NewCron(dcron.WithSLog(&LoggerAdapter{lgr}))
	job2 := dcron.NewJob("A local job", "*/15 * * * * *", func(ctx context.Context) error {
		// do something
		return nil
	})
	if err := cron.AddJobs(job2); err != nil {
		log.Fatal(err)
	}

	type LoggerAdapter struct {
		lgr *logger.Logger
	}
	
	func (la *LoggerAdapter) ErrorContext(ctx context.Context, msg string, args ...any) {
		la.lgr.With(args...).Error(msg)
	}
	
	func (la *LoggerAdapter) InfoContext(ctx context.Context, msg string, args ...any) {
		la.lgr.With(args...).Debug(msg) // set the desired log level here
	}
```

To set a custom log level per specific job
```go
	cron := dcron.NewCron()
	job2 := dcron.NewJob("A local job", "*/15 * * * * *", func(ctx context.Context) error {
		// do something
		return nil
	}, dcron.WithJobSLog(&LoggerAdapter{lgr}))
	if err := cron.AddJobs(job2); err != nil {
		log.Fatal(err)
	}

	type LoggerAdapter struct {
		lgr *logger.Logger
	}
	
	func (la *LoggerAdapter) ErrorContext(ctx context.Context, msg string, args ...any) {
		la.lgr.With(args...).Error(msg)
	}
	
	func (la *LoggerAdapter) InfoContext(ctx context.Context, msg string, args ...any) {
		la.lgr.With(args...).Debug(msg) // set the desired log level here
	}
```

Of course, in case when you use zap logger instead of slog, you can write an adapter for its sugared api, which provides structured capabilities:
```go
	cron := dcron.NewCron(dcron.WithSLog(&StructuredZapLoggerAdapter{lgr}))
	job2 := dcron.NewJob("A local job", "*/15 * * * * *", func(ctx context.Context) error {
		// do something
		return nil
	})
	if err := cron.AddJobs(job2); err != nil {
		log.Fatal(err)
	}

type StructuredZapLoggerAdapter struct {
	lgr *zap.SugaredLogger
}

func (la *StructuredZapLoggerAdapter) ErrorContext(ctx context.Context, msg string, args ...any) {
	la.lgr.With(args...).Error(msg)
}

func (la *StructuredZapLoggerAdapter) InfoContext(ctx context.Context, msg string, args ...any) {
	la.lgr.With(args...).Info(msg)
}
```

# Plugins
## Redis lock

There is already implemented RedisLock:


Now you can create a cron with that:

```go
import (
	redisLock "github.com/nkonev/dcron/plugin/lock/redis"
	redisV9 "github.com/redis/go-redis/v9"
	"log/slog"
)

func main() {
	redisClient := redisV9.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	
	lgr := slog.Default()
	
	cron := dcron.NewCron(
		redisLock.WithLock(redisClient, redisLock.WithSLog(lgr)),
		dcron.WithSLog(lgr),
	)
}
```

Then, create a job and add it to the cron.

```go
	job1 := dcron.NewJob("Job1", "*/15 * * * * *", func(ctx context.Context) error {
		if task, ok := dcron.TaskFromContext(ctx); ok {
			lgr.InfoContext(ctx, "run:", "dcron_task_spec", task.Job.Spec(), dcron.SlogKeyTaskName, task.Key)
		}
		// do something
		return nil
	}, redisLock.WithLockTTL(time.Minute))
	if err := cron.AddJobs(job1); err != nil {
		panic(err)
	}
```

Finally, start the cron:

```go
	cron.Start()
	lgr.Info("cron started")
	time.Sleep(time.Minute)
	<-cron.Stop().Done()
```

## OTeL Tracing

```go
	import (
		otelTrace "github.com/nkonev/dcron/plugin/trace/otel"
		"go.opentelemetry.io/otel"
	)

	cron := dcron.NewCron()
	job2 := dcron.NewJob("A local job", "*/15 * * * * *", func(ctx context.Context) error {
		// do something
		return nil
	}, otelTrace.WithTracing(otel.Tracer("scheduler/a-local-job"), "aLocalJobSpan"))
	if err := cron.AddJobs(job2); err != nil {
		log.Fatal(err)
	}

	cron.Start()
	log.Info("cron started")
	time.Sleep(time.Minute)
	<-cron.Stop().Done()

```