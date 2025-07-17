BINARY_NAME = taproom
SRC = $(wildcard *.go)
GOBIN = $(HOME)/.local/bin
TARGET_OS = darwin
TARGET_ARCH = arm64 amd64

all: build

$(BINARY_NAME): $(SRC)
	go mod tidy
	go build -o $(BINARY_NAME)

build: $(BINARY_NAME)

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
	rm -rf release

install: build
	GOBIN=$(GOBIN) go install

fmt:
	gofmt -w -s $(SRC)

vet:
	go vet $(SRC)

test:
	go test

release:
	mkdir -p release
	for arch in $(TARGET_ARCH); do \
		GOOS=$(TARGET_OS) GOARCH=$$arch go build -o release/$(BINARY_NAME)-$(TARGET_OS)-$$arch; \
		tar -C release -czf release/$(BINARY_NAME)-$(TARGET_OS)-$$arch.tar.gz $(BINARY_NAME)-$(TARGET_OS)-$$arch; \
		rm release/$(BINARY_NAME)-$(TARGET_OS)-$$arch; \
	done
	(cd release && shasum -a 256 *.tar.gz > checksum.txt)

.PHONY: all build run clean install fmt vet test release
