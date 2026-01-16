.PHONY: all build test clean install lint fmt vet

BINARY_NAME=limiter
INSTALL_PATH=/usr/local/bin

all: test build

build:
	go build -v -ldflags="-s -w" -o $(BINARY_NAME)

test:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f coverage.txt
	rm -f limiter-*-*

install: build
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run

# Build for multiple Linux platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BINARY_NAME)-linux-amd64
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BINARY_NAME)-linux-arm64
