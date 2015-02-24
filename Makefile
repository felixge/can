VERSION = $(shell git describe --tags --dirty --always)
GOPATH := $(CURDIR)/Godeps/_workspace:$(GOPATH)
PATH := $(GOPATH)/bin:$(PATH)

all: bin bin/can

bin:
	mkdir -p bin

bin/can: bin
	go build -o $@ -ldflags "-X main.Version $(VERSION)" ./cmd/can

test:
	go test -v ./...

.PHONY: all bin/can test
