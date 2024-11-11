package dcron

import "context"

//go:generate go get go.uber.org/mock/mockgen
//go:generate go run go.uber.org/mock/mockgen -source=lock.go -destination mock_dcron/lock.go
//go:generate go mod tidy

// Lock provides distributed lock operation for dcron,
// it can be implemented easily via Redis/SQL and so on.
type Lock interface {
	// Lock stores the key/value and return true if the key is not existed,
	// or does nothing and return false.
	// Note that the key/value should be kept for at least one minute.
	// For example, `SetNX(key, value, time.Minute)` via redis.
	Lock(ctx context.Context, key, value string) bool

	// Unlock removes the key/value,
	// or does nothing.
	Unlock(ctx context.Context, key, value string)
}
