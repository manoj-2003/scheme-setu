.PHONY: help run offline warm plan test lint build clean

BIN := bin
FIXTURE_CACHE := fixtures/serp-cache.sqlite

help:
	@echo "run      - start the API (live mode, needs SERPAPI_KEY)"
	@echo "offline  - start the API against the committed fixture cache, zero credits"
	@echo "plan     - print the SerpApi queries the demo personas would issue"
	@echo "warm     - fetch those queries into $(FIXTURE_CACHE) (spends credits)"
	@echo "test     - go test ./..."
	@echo "lint     - go vet ./... + gofmt check"
	@echo "build    - build both binaries into $(BIN)/"

run:
	go run ./cmd/server

# The mode used for the demo recording: no network, no credits, instant answers.
offline:
	SERP_MODE=cache SERP_CACHE_PATH=$(FIXTURE_CACHE) go run ./cmd/server

plan:
	go run ./cmd/warm -dry-run

warm:
	go run ./cmd/warm -cache $(FIXTURE_CACHE)

test:
	go test ./...

lint:
	go vet ./...
	@gofmt -l . | tee /dev/stderr | (! read)

build:
	go build -o $(BIN)/server ./cmd/server
	go build -o $(BIN)/warm ./cmd/warm

clean:
	rm -rf $(BIN) data
