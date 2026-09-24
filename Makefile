MODULE  := github.com/saaranganand/ledsure
BINARY  := ledsured
BIN_DIR := bin

.PHONY: all
all: lint test build

.PHONY: build
build:
	go build -trimpath -o $(BIN_DIR)/$(BINARY) ./cmd/ledsured

.PHONY: run
run:
	go run ./cmd/ledsured

.PHONY: up
up:
	docker compose up --build

.PHONY: down
down:
	docker compose down -v

.PHONY: logs
logs:
	docker compose logs -f ledsured

.PHONY: psql
psql:
	docker compose exec db psql -U ledsure -d ledsure

.PHONY: test
test:
	go test -race ./...

.PHONY: test-integration
test-integration:
	go test -race -tags=integration ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -l -w .
	goimports -l -w .

.PHONY: tidy
tidy:
	go mod tidy

# applies migrations against the postgres from `make up`, then starts the server
# ctrl+c once you see "migrations applied" — `make up` already does this automatically
.PHONY: migrate
migrate:
	LEDSURE_MIGRATE_ON_START=true \
	LEDSURE_DATABASE_URL=$${LEDSURE_DATABASE_URL:-postgres://ledsure:ledsure@localhost:5432/ledsure?sslmode=disable} \
	go run ./cmd/ledsured

.PHONY: ci
ci: lint test test-integration
