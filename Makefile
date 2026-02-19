.PHONY: build run clean test migrate

BINARY=piperelay
BUILD_DIR=./bin

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -o $(BUILD_DIR)/$(BINARY) ./cmd/piperelay

run: build
	$(BUILD_DIR)/$(BINARY) serve

clean:
	rm -rf $(BUILD_DIR) ./data

test:
	go test ./...

migrate: build
	$(BUILD_DIR)/$(BINARY) migrate

dev:
	@mkdir -p ./data
	CGO_ENABLED=1 go run ./cmd/piperelay serve --config ./piperelay.yaml
