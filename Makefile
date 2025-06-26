BINARY_NAME = taproom
SRC = $(wildcard *.go)
GOBIN = $(HOME)/.local/bin

all: build

$(BINARY_NAME): $(SRC)
	go mod tidy
	go build -o $(BINARY_NAME)

build: $(BINARY_NAME)

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)

install: build
	GOBIN = $(GOBIN) go install

fmt:
	gofmt -w -s $(SRC)

vet:
	go vet $(SRC)

.PHONY: all build run clean install fmt vet
