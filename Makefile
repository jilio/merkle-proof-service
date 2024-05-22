#!/usr/bin/make -f

BRANCH:=$(shell git rev-parse --abbrev-ref HEAD)
VERSION ?= $(shell git tag --sort=committerdate -l 'v*' | tail -1)
COMMIT := $(shell git log -1 --format='%h')
BINARY_NAME = galacticad-merkle
BUILDDIR ?= $(CURDIR)/build
BINDIR ?= $(GOPATH)/bin
DOCKER_IMAGE_NAME = galactica-corp/merkle-proof-service
BUILD_CMD_DIR = ./cmd/galacticad-merkle/

# if version is empty, set it to "dev"
VERSION := $(if $(VERSION),$(VERSION),v0.0.0)

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

build: ## build the binary
	CGO_ENABLED=$(f) go build -tags "$(build_tags)" -ldflags "$(ldflags)" -o $(BUILDDIR)/$(BINARY_NAME) $(BUILD_CMD_DIR)

build-linux: ## build for linux
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -tags "$(build_tags)" -ldflags "$(ldflags)" -o $(BUILDDIR)/$(BINARY_NAME) $(BUILD_CMD_DIR)

install: ## install the binary
	CGO_ENABLED=$(CGO_ENABLED) go install -tags "$(build_tags)" -ldflags "$(ldflags)" $(BUILD_CMD_DIR)

docker-build: ## build the docker image
	docker build --build-arg DB_BACKEND=$(DB_BACKEND) -t $(DOCKER_IMAGE_NAME):sha-$(COMMIT) .

docker-push: docker-build ## push the docker image
	docker push $(DOCKER_IMAGE_NAME):sha-$(COMMIT)
	docker tag $(DOCKER_IMAGE_NAME):sha-$(COMMIT) $(DOCKER_IMAGE_NAME):$(BRANCH)
	docker push $(DOCKER_IMAGE_NAME):$(BRANCH)
ifeq ($(BRANCH),main)
	docker tag $(DOCKER_IMAGE_NAME):sha-$(COMMIT) $(DOCKER_IMAGE_NAME):latest
	docker push $(DOCKER_IMAGE_NAME):latest
endif
ifneq ($(shell git tag --contains ${COMMIT}),)
	docker tag $(DOCKER_IMAGE_NAME):sha-$(COMMIT) $(DOCKER_IMAGE_NAME):$(VERSION)
	docker push $(DOCKER_IMAGE_NAME):$(VERSION)
endif

clean: ## remove the binary
	rm -f $(BUILDDIR)/$(BINARY_NAME)

help: ## Show this help
	@printf "\033[33m%s:\033[0m\n" 'Available commands'
	@awk 'BEGIN {FS = ":.*?## "} /^[[:alpha:][:punct:]]+:.*?## / {printf "  \033[32m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
