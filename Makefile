APP := goose-go

.PHONY: run build test lint fmt tidy clean check smoke eval

run:
	go run ./cmd/$(APP)

build:
	mkdir -p bin
	go build -o bin/$(APP) ./cmd/$(APP)

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

tidy:
	go mod tidy

check: fmt test lint

smoke: run

eval:
	@echo "eval harness not implemented yet"
	@exit 1

clean:
	rm -rf bin coverage.out
