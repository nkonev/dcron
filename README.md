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
