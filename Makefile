BIN := watch-tower

.PHONY: build test lint

build:
	GO111MODULE=on go build -trimpath -o bin/$(BIN) ./cmd/watch-tower

test:
	GO111MODULE=on go test ./...

lint:
	GO111MODULE=on go vet ./...

