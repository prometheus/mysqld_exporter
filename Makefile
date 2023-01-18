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

# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_ARCHS ?= amd64 armv7 arm64

all: vet

include Makefile.common

STATICCHECK_IGNORE =

DOCKER_IMAGE_NAME ?= mysqld-exporter

test-docker:
	@echo ">> testing docker image"
	./test_image.sh "$(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)" 9104

.PHONY: test-docker

GO_BUILD_LDFLAGS = -X github.com/prometheus/common/version.Version=$(shell cat VERSION) -X github.com/prometheus/common/version.Revision=$(shell git rev-parse HEAD) -X github.com/prometheus/common/version.Branch=$(shell git describe --always --contains --all) -X github.com/prometheus/common/version.BuildUser= -X github.com/prometheus/common/version.BuildDate=$(shell date +%FT%T%z) -s -w

export PMM_RELEASE_PATH?=.

release:
	go build -ldflags="$(GO_BUILD_LDFLAGS)" -o $(PMM_RELEASE_PATH)/mysqld_exporter
