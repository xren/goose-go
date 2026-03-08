APP := goose-go
ARCHCHECK := archcheck

.PHONY: run build test lint fmt tidy clean check archcheck smoke eval

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

fmt:
	go fmt ./...

tidy:
	go mod tidy

check: fmt test lint archcheck

smoke: run

eval:
	go test ./internal/evals -v

clean:
	rm -rf bin coverage.out
