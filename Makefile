.PHONY: build run test lint migrate-up migrate-down docker-up docker-down clean \
       console-deps console-build console-dev build-console

BINARY_NAME=quorum
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run ./...

migrate-up:
	migrate -path migrations -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://quorum:quorum@localhost:5432/quorum?sslmode=disable" down 1

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

clean:
	rm -rf $(BUILD_DIR)
	rm -rf console/frontend/dist
	go clean
