.PHONY: build run test docker clean

SRC := $(shell find . -type f -name '*.go' -not -path '*/vendor/*' -print)
DEPS := $(shell find vendor -type f -print) go.mod go.sum
BINDIR := bin

.PHONY: all
all: build

.PHONY: build
build: $(BINDIR)/exporter-keeplink

$(BINDIR):
	mkdir -p '$@'
$(BINDIR)/exporter-keeplink: $(SRC) $(DEPS)
	go build -o $(BINDIR)/exporter-keeplink $(SRC)

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: clean
clean:
	rm -r $(BINDIR)
	go clean

