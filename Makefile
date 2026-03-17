.PHONY: build run test test-integration test-all lint migrate-up migrate-down docker-up docker-down clean \
       console-deps console-build console-dev build-console \
       embed-deps embed-build embed-dev build-embed build-all \
       seed demo

BINARY_NAME=quorum
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v -race -cover ./...

test-integration:
	go test -v -race -tags integration ./internal/store/...

test-all: test test-integration

lint:
	golangci-lint run ./...

migrate-up:
	psql "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable" -c "CREATE SCHEMA IF NOT EXISTS quorum;"
	migrate -path migrations/postgres -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable&search_path=quorum" up

migrate-down:
	migrate -path migrations/postgres -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable&search_path=quorum" down 1

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

console-deps:
	cd console/frontend && npm install

console-build: console-deps
	cd console/frontend && npm run build

console-dev:
	cd console/frontend && npm run dev

build-console: console-build
	go build -tags console -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

embed-deps:
	cd widgets/frontend && npm install

embed-build: embed-deps
	cd widgets/frontend && npm run build

embed-dev:
	cd widgets/frontend && npm run dev

build-embed: embed-build
	go build -tags embed -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

build-all: console-build embed-build
	go build -tags "console,embed" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

seed:
	sh scripts/seed.sh

demo:
	docker compose --profile demo up --build

clean:
	rm -rf $(BUILD_DIR)
	rm -rf console/frontend/dist
	rm -rf widgets/frontend/dist
	go clean
