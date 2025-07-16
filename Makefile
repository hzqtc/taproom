BINARY_NAME = taproom
SRC = $(wildcard *.go)
GOBIN = $(HOME)/.local/bin
VERSION = 0.1.7
LD_FLAGS = "-X main.version=$(VERSION)"
TARGET_OS = darwin
TARGET_ARCH = arm64 amd64

all: build

$(BINARY_NAME): $(SRC)
	go mod tidy
	go build -ldflags $(LD_FLAGS) -o $(BINARY_NAME)

build: $(BINARY_NAME)

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
	rm -rf release

install: build
	GOBIN=$(GOBIN) go install -ldflags $(LD_FLAGS)

fmt:
	gofmt -w -s $(SRC)

vet:
	go vet $(SRC)

release:
	mkdir -p release
	for arch in $(TARGET_ARCH); do \
		GOOS=$(TARGET_OS) GOARCH=$$arch go build -ldflags $(LD_FLAGS) -o release/$(BINARY_NAME)-$(TARGET_OS)-$$arch; \
		tar -C release -czf release/$(BINARY_NAME)-$(TARGET_OS)-$$arch.tar.gz $(BINARY_NAME)-$(TARGET_OS)-$$arch; \
		rm release/$(BINARY_NAME)-$(TARGET_OS)-$$arch; \
	done
	(cd release && shasum -a 256 *.tar.gz > checksum.txt)

.PHONY: all build run clean install fmt vet release
