# Copyright 2015 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO           := go
FIRST_GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
PROMU        := bin/promu
pkgs          = $(shell $(GO) list ./...)

PREFIX              ?= $(shell pwd)
BIN_DIR             ?= $(shell pwd)
DOCKER_IMAGE_NAME   ?= mysqld-exporter
DOCKER_IMAGE_TAG    ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))
TMPDIR              ?= $(shell dirname $(shell mktemp)/)

default: help

all: format build test-short

env-up:           ## Start MySQL and copy ssl certificates to /tmp
	@docker-compose up -d
	@sleep 5
	@docker container cp mysqld_exporter_db:/var/lib/mysql/client-cert.pem $(TMPDIR)
	@docker container cp mysqld_exporter_db:/var/lib/mysql/client-key.pem $(TMPDIR)
	@docker container cp mysqld_exporter_db:/var/lib/mysql/ca.pem $(TMPDIR)

env-down:         ## Stop MySQL and clean up certs
	@docker-compose down
	@rm ${TMPDIR}/client-cert.pem ${TMPDIR}/client-key.pem ${TMPDIR}/ca.pem

style:            ## Check the code style
	@echo ">> checking code style"
	@! gofmt -d $(shell find . -name '*.go' -print) | grep '^'

test-short:       ## Run short tests
	@echo ">> running short tests"
	@$(GO) test -short -race $(pkgs)

test:             ## Run all tests
	@echo ">> running tests"
	@$(GO) test -race $(pkgs)

format:           ## Format the code
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

FILES = $(shell find . -type f -name '*.go')

fumpt:            ## Format source code using fumpt and fumports.
	@gofumpt -w -s $(FILES)
	@gofumports -local github.com/percona/mysqld_exporter -l -w $(FILES)

vet:              ## Run vet
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)

build: promu      ## Build binaries
	@echo ">> building binaries"
	@$(PROMU) build --prefix $(PREFIX)

tarball: promu    ## Build release tarball
	@echo ">> building release tarball"
	@$(PROMU) tarball --prefix $(PREFIX) $(BIN_DIR)

docker:           ## Build docker image
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" .

promu:            ## Install promu
	@GOOS=$(shell uname -s | tr A-Z a-z) \
		GO111MODULE=on \
		GOARCH=$(subst x86_64,amd64,$(patsubst i%86,386,$(shell uname -m))) \
		$(GO) build -modfile=tools/go.mod -o bin/promu github.com/prometheus/promu

help:             ## Display this help message.
	@echo "$(TMPDIR)"
	@echo "Please use \`make <target>\` where <target> is one of:"
	@grep '^[a-zA-Z]' $(MAKEFILE_LIST) | \
        awk -F ':.*?## ' 'NF==2 {printf "  %-26s%s\n", $$1, $$2}'

export PMM_RELEASE_PATH?=.

release:
	go build -ldflags="$(GO_BUILD_LDFLAGS)" -o $(PMM_RELEASE_PATH)/mysqld_exporter

.PHONY: all style format build test vet tarball docker promu env-up env-down help default
