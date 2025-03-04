# Copyright (c) 2024, Shanghai Iluvatar CoreX Semiconductor Co., Ltd.
# All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

DOCKER   ?= docker
MKDIR    ?= mkdir

VERSION  ?= v0.1.0
REGISTRY ?= iluvatarcorex

ifeq ($(IMAGE_NAME),)
IMAGE_NAME = $(REGISTRY)/ix-feature-discovery:$(VERSION)
endif

GIT_COMMIT ?= $(shell git describe --match="" --dirty --long --always --abbrev=40 2> /dev/null || echo "")

MODULE := gitee.com/deep-spark/ix-feature-discovery
BUILD_DIR := build
ABS_BUILD_DIR := $(CURDIR)/build
TARGET := ix-feature-discovery

MAKE_TARGETS := all build binary image cmds
GO_TARGETS := fmt vet vendor generate test coverage

CMDS := $(patsubst ./cmd/%/,%,$(sort $(dir $(wildcard ./cmd/*/))))
CMD_TARGETS := $(patsubst %,cmd-%, $(CMDS))

TARGETS := $(MAKE_TARGETS) $(GO_TARGETS) $(CMD_TARGETS)

.PHONY: $(TARGETS)

all: image binary

binary: vendor cmds

BUILDFLAGS = -ldflags "-s -w '-extldflags=-Wl,-undefined,dynamic_lookup'"
COMMAND_BUILD_OPTIONS = -o $(BUILD_DIR)/$(*)

cmds: $(CMD_TARGETS)
$(CMD_TARGETS): cmd-%:
	$(MKDIR) -p $(BUILD_DIR)
	go build $(BUILDFLAGS) $(COMMAND_BUILD_OPTIONS) $(MODULE)/cmd/$(*)

COREX_PATH ?= /usr/local/corex
DEPENDS_LIBS := libixml.so \
           	libcuda.so \
	        libcuda.so.1 \
	        libcudart.so \
	        libcudart.so.10.2 \
	        libcudart.so.10.2.89 \
	        libixthunk.so

# Make container image
image: binary
	$(MKDIR) -p $(BUILD_DIR)/lib64
	$(foreach lib, $(DEPENDS_LIBS), cp -P $(COREX_PATH)/lib64/$(lib) $(BUILD_DIR)/lib64;)
	$(DOCKER) build -t $(IMAGE_NAME) \
		--build-arg CMDS="$(BUILD_DIR)/$(CMDS)" \
		--build-arg LIB_DIR="$(BUILD_DIR)/lib64" \
		-f deployment/container/Dockerfile \
		.

# Apply go fmt to the codebase
fmt: vendor
	go fmt $(MODULE)/...

vet: vendor
	go vet $(MODULE)/...

vendor:
	go mod tidy
	go mod vendor
	go mod verify

generate:
	go generate $(MODULE)/...		

COVERAGE_FILE := coverage.out
test: build cmds
	go test -coverprofile=$(COVERAGE_FILE) $(MODULE)/cmd/... $(MODULE)/internal/... $(MODULE)/api/...

coverage: test
	cat $(COVERAGE_FILE) | grep -v "_mock.go" > $(COVERAGE_FILE).no-mocks
	go tool cover -func=$(COVERAGE_FILE).no-mocks
