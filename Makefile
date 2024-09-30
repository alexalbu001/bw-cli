BINARY_NAME=bw-cli
VERSION=$(shell git describe --tags --always --dirty)

.PHONY: all clean build tar

all: build tar

clean:
	rm -f $(BINARY_NAME)*

build:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 -ldflags "-X main.version=$(VERSION)"
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 -ldflags "-X main.version=$(VERSION)"

tar:
	tar -czf $(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	tar -czf $(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
