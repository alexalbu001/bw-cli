BINARY_NAME=bw-cli
VERSION=$(shell git describe --tags --always --dirty)

.PHONY: all clean build

all: build

clean:
	rm -f bw-cli*

build:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 -ldflags "-X main.version=$(VERSION)"
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 -ldflags "-X main.version=$(VERSION)"
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe -ldflags "-X main.version=$(VERSION)"
