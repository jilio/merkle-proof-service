#!/usr/bin/make -f

VERSION ?= $(shell echo $(shell git describe --tags `git rev-list --tags="v*" --max-count=1`) | sed 's/^//')
COMMIT := $(shell git log -1 --format='%H')
BINARY_NAME = galacticad-merkle
BUILDDIR ?= $(CURDIR)/build
BINDIR ?= $(GOPATH)/bin
DOCKER_IMAGE_NAME = Galactica-corp/merkle-proof-service
BUILD_CMD_DIR = ./cmd/galacticad-merkle/

# if version is empty, set it to "dev"
ifeq ($(VERSION),)
	VERSION = v0.0.0
endif

# DB backend to use for the K/V store
# available options: goleveldb, cleveldb, memdb, boltdb, rocksdb, badgerdb, pebbledb
DB_BACKEND ?= goleveldb

# process build tags
build_tags = netgo
CGO_ENABLED = 0

ifeq ($(DB_BACKEND),cleveldb)
	build_tags += cleveldb
	CGO_ENABLED=1
endif

ifeq ($(DB_BACKEND),boltdb)
	build_tags += boltdb
endif

ifeq ($(DB_BACKEND),rocksdb)
	build_tags += rocksdb
	CGO_ENABLED=1
endif

ifeq ($(DB_BACKEND),pebbledb)
	build_tags += pebbledb
endif

build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

# replace build_tags all spaces to commas
whitespace :=
whitespace += $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))

ldflags = -X github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd.AppName=$(BINARY_NAME) \
		  -X github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd.Version=$(VERSION) \
		  -X github.com/Galactica-corp/merkle-proof-service/cmd/galacticad-merkle/cmd.GitCommit=$(COMMIT)


.PHONY: build install clean build-linux docker-build help

# build command
build:
	CGO_ENABLED=$(f) go build -tags "$(build_tags)" -ldflags "$(ldflags)" -o $(BUILDDIR)/$(BINARY_NAME) $(BUILD_CMD_DIR)

# build for linux
build-linux:
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -tags "$(build_tags)" -ldflags "$(ldflags)" -o $(BUILDDIR)/$(BINARY_NAME) $(BUILD_CMD_DIR)

# install command
install:
	CGO_ENABLED=$(CGO_ENABLED) go install -tags "$(build_tags)" -ldflags "$(ldflags)" $(BUILD_CMD_DIR)

# Docker build command
docker-build:
	docker build --build-arg DB_BACKEND=$(DB_BACKEND) -t $(DOCKER_IMAGE_NAME):$(VERSION) .

# clean command
clean:
	rm -f $(BUILDDIR)/$(BINARY_NAME)

help:
	@echo "build - build the binary"
	@echo "build-linux - build the binary for linux"
	@echo "install - install the binary"
	@echo "docker-build - build the docker image"
	@echo "clean - remove the binary"
	@echo "help - display this help message"
