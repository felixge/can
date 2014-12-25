VERSION = $(shell git describe --tags --dirty --always)
GOPATH := $(CURDIR)/Godeps/_workspace:$(GOPATH)
PATH := $(GOPATH)/bin:$(PATH)

all: bin bin/gkv

bin:
	mkdir -p bin

bin/gkv: bin
	go build -o $@ -ldflags "-X main.Version $(VERSION)" ./cmd/gkv

test:
	go test -v ./...

.PHONY: all bin/gkv test
