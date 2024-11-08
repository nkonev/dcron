```bash
go install github.com/golang/mock/mockgen@v1.6.0
mockgen -source=entry_getter.go -destination mock_dcron/entry_getter.go
mockgen -source=atomic.go -destination mock_dcron/atomic.go
go test ./... -count=1 -test.v -p 1
```