generate:
	@go generate

.PHONY: test
test:
	@go test ./... -count=1 -test.v -p 1

.PHONY: test-race
test-race:
	@go test -race -coverprofile=coverage.out -covermode=atomic

.PHONY: format
format:
	@gofmt -l .

.PHONY: vet
vet:
	@go vet -v ./...
