.DEFAULT_GOAL := build

GOFMT=gofmt
GC=go build
VERSION := $(shell git describe --abbrev=4 --dirty --always --tags)
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
