APP := goose-go
ARCHCHECK := archcheck
REPOCHECK := repocheck

.PHONY: run build test lint fmt tidy clean check archcheck repocheck smoke eval

run:
	go run ./cmd/$(APP)

build:
	mkdir -p bin
	go build -o bin/$(APP) ./cmd/$(APP)

test:
	go test ./...

lint:
	golangci-lint run

archcheck:
	go run ./cmd/$(ARCHCHECK)

repocheck:
	go run ./cmd/$(REPOCHECK)

fmt:
	go fmt ./...

tidy:
	go mod tidy

check: fmt test lint archcheck repocheck

smoke: run

eval:
	go test ./internal/evals -v

clean:
	rm -rf bin coverage.out
