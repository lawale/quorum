.PHONY: build run test lint migrate-up migrate-down docker-up docker-down clean

BINARY_NAME=maker-checker
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
	migrate -path migrations -database "postgres://makerchecker:makerchecker@localhost:5432/makerchecker?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://makerchecker:makerchecker@localhost:5432/makerchecker?sslmode=disable" down 1

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

clean:
	rm -rf $(BUILD_DIR)
	go clean
