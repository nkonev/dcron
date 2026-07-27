package dcron

import "github.com/robfig/cron/v3"

//go:generate go get go.uber.org/mock/mockgen@v0.6.0
//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=entry_getter.go -destination mock_dcron/entry_getter.go
//go:generate go mod tidy

type entryGetter interface {
	Entry(id cron.EntryID) cron.Entry
}
