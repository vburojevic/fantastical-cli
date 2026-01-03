.PHONY: test cover fmt lint release

test:
	go test ./...

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

fmt:
	gofmt -w .

lint:
	go vet ./...
	golangci-lint run ./...

release:
	goreleaser release --clean
