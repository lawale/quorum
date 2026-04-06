.PHONY: build run test test-integration test-all lint \
       migrate-up migrate-down migrate-version \
       docker-up docker-down clean \
       console-deps console-build console-dev build-console \
       embed-deps embed-build embed-dev build-embed build-all \
       seed demo \
       docker-build docker-build-api docker-build-console docker-build-all \
       docker-push docker-push-api docker-push-console docker-push-all \
       docker-buildx docker-buildx-api docker-buildx-console docker-buildx-all \
       quickstart

BINARY_NAME=quorum
BUILD_DIR=bin
LDFLAGS=-s -w

# Docker image settings
IMAGE_NAME ?= quorum
REGISTRY ?=
IMAGE_TAG ?= $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD)
PLATFORM ?= linux/amd64,linux/arm64

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v -race -cover ./...

test-integration:
	go test -v -race -tags integration ./internal/store/...

test-all: test test-integration

lint:
	golangci-lint run ./...

migrate-up: build
	./$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml -migrate up

migrate-down: build
	./$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml -migrate down

migrate-version: build
	./$(BUILD_DIR)/$(BINARY_NAME) -config config.yaml -migrate version

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# Docker build targets
docker-build: docker-build-api docker-build-console docker-build-all

docker-build-api:
	docker build -f Dockerfile -t $(REGISTRY)$(IMAGE_NAME):$(IMAGE_TAG) .

docker-build-console:
	docker build -f Dockerfile.console -t $(REGISTRY)$(IMAGE_NAME)-console:$(IMAGE_TAG) .

docker-build-all:
	docker build -f Dockerfile.all -t $(REGISTRY)$(IMAGE_NAME)-all:$(IMAGE_TAG) .

# Docker push targets
docker-push: docker-push-api docker-push-console docker-push-all

docker-push-api:
	docker push $(REGISTRY)$(IMAGE_NAME):$(IMAGE_TAG)

docker-push-console:
	docker push $(REGISTRY)$(IMAGE_NAME)-console:$(IMAGE_TAG)

docker-push-all:
	docker push $(REGISTRY)$(IMAGE_NAME)-all:$(IMAGE_TAG)

# Docker buildx (multi-platform build + push)
docker-buildx: docker-buildx-api docker-buildx-console docker-buildx-all

docker-buildx-api:
	docker buildx build --platform $(PLATFORM) -f Dockerfile -t $(REGISTRY)$(IMAGE_NAME):$(IMAGE_TAG) --push .

docker-buildx-console:
	docker buildx build --platform $(PLATFORM) -f Dockerfile.console -t $(REGISTRY)$(IMAGE_NAME)-console:$(IMAGE_TAG) --push .

docker-buildx-all:
	docker buildx build --platform $(PLATFORM) -f Dockerfile.all -t $(REGISTRY)$(IMAGE_NAME)-all:$(IMAGE_TAG) --push .

console-deps:
	cd console/frontend && npm install

console-build: console-deps
	cd console/frontend && npm run build

console-dev:
	cd console/frontend && npm run dev

build-console: console-build
	go build -trimpath -ldflags="$(LDFLAGS)" -tags console -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

embed-deps:
	cd widgets/frontend && npm install

embed-build: embed-deps
	cd widgets/frontend && npm run build

embed-dev:
	cd widgets/frontend && npm run dev

build-embed: embed-build
	go build -trimpath -ldflags="$(LDFLAGS)" -tags embed -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

build-all: console-build embed-build
	go build -trimpath -ldflags="$(LDFLAGS)" -tags "console,embed" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

seed:
	sh scripts/seed.sh

demo:
	docker compose --profile demo up --build

quickstart:
	bash quickstart.sh

clean:
	rm -rf $(BUILD_DIR)
	rm -rf console/frontend/dist
	rm -rf widgets/frontend/dist
	go clean
