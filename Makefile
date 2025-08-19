BINARY_NAME = taproom
GOBIN = $(HOME)/.local/bin
TARGET_OS = darwin
TARGET_ARCH = arm64 amd64
VERSION = $(shell cat .version)
ASSETS  = $(wildcard release/*)
RELEASE_NOTE = .release-note.md

all: build

$(BINARY_NAME):
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

test:
	go test ./...

release:
	mkdir -p release
	for arch in $(TARGET_ARCH); do \
		echo "Building release/$(BINARY_NAME)-$$os-$$arch"; \
		GOOS=$(TARGET_OS) GOARCH=$$arch go build -o release/$(BINARY_NAME)-$(TARGET_OS)-$$arch; \
		tar -C release -czf release/$(BINARY_NAME)-$(TARGET_OS)-$$arch.tar.gz $(BINARY_NAME)-$(TARGET_OS)-$$arch; \
		rm release/$(BINARY_NAME)-$(TARGET_OS)-$$arch; \
	done
	(cd release && shasum -a 256 *.tar.gz > checksum.txt)

gh-release: release
	git tag $(VERSION)
	git push origin $(VERSION)
	$(EDITOR) $(RELEASE_NOTE)
	gh release create $(VERSION) $(ASSETS) \
		--title "$(VERSION)" \
		--notes-file $(RELEASE_NOTE)
	rm $(RELEASE_NOTE)

.PHONY: all build run clean install fmt vet test release
