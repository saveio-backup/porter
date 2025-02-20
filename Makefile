.DEFAULT_GOAL := build

GOFMT=gofmt
GC=go build
LAST_VERSION := $(shell git describe --match "v*" --always --tags)
VERSION_PARTS      := $(subst ., ,$(LAST_VERSION))
MAJOR              := $(word 1,$(VERSION_PARTS))
MINOR              := $(word 2,$(VERSION_PARTS))
MICRO              := $(word 3,$(VERSION_PARTS))
MICRO_PARTS := $(subst -, ,$(MICRO))
MICRO_FIRST :=  $(word 1,$(MICRO_PARTS))
NEXT_MICRO         := $(shell echo $$(($(MICRO_FIRST)+1)))
VERSION := $(MAJOR).$(MINOR).$(NEXT_MICRO)
Minversion := $(shell date)
IDENTIFIER= $(GOOS)-$(GOARCH)
BUILD_PORTER_PAR = -ldflags "-X github.com/saveio/porter/common.Version=$(VERSION)" #-race

help:          ## Show available options with this Makefile
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -v grep | awk 'BEGIN { FS = ":.*?##" }; { printf "%-15s  %s\n", $$1,$$2 }'

.PHONY: build
build: clean
	$(GC) $(BUILD_PORTER_PAR) -o porter main.go

.PHONY: glide
glide:   ## Installs glide for go package management
	@ mkdir -p $$(go env GOPATH)/bin
	@ curl https://glide.sh/get | sh;

vendor: glide.yaml glide.lock
	@ glide install

.PHONY: clean
clean:
	rm -rf porter
	rm -rf build/
	rm -rf *.log

.PHONY: pb
pb:
	protoc -I=. -I=$(GOPATH)/src -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf --gogoslick_out=. pb/*.proto

.PHONY: deploy
deploy:
	deploy_test porter
