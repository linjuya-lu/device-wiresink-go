.PHONY: build clean docker
ENABLE_FULL_RELRO=true
ENABLE_PIE=false
MICROSERVICES=cmd/device-wiresink
.PHONY: $(MICROSERVICES)

ARCH ?= aarch64
ifeq ($(ARCH),aarch64)
  GOOS   ?= linux
  GOARCH ?= arm64
else ifeq ($(ARCH),arm64)
  GOOS   ?= linux
  GOARCH ?= arm64
else ifneq (,$(filter armv7l armhf arm,$(ARCH)))
  GOOS   ?= linux
  GOARCH ?= arm
  GOARM  ?= 7
else ifneq (,$(filter x86_64 amd64,$(ARCH)))
  GOOS   ?= linux
  GOARCH ?= amd64
else
  GOOS   ?= $(shell go env GOOS)
  GOARCH ?= $(shell go env GOARCH)
endif

export GOOS GOARCH GOARM
DOCKERS=docker_device_wiresink_go_arm64
.PHONY: $(DOCKERS)
VERSION=$(shell echo 1.0.0)
GIT_SHA=$(shell git rev-parse HEAD)
SDKVERSION=$(shell cat ./go.mod | grep 'github.com/edgexfoundry/device-sdk-go/v4 v' | sed 's/require//g' | awk '{print $$2}')

ifeq ($(ENABLE_FULL_RELRO), true)
	ENABLE_FULL_RELRO_GOFLAGS = -bindnow
endif
GOFLAGS=-ldflags "-s -w -X github.com/edgexfoundry/device-wiresink-go.Version=$(VERSION) \
                  -X github.com/edgexfoundry/device-sdk-go/v4/internal/common.SDKVersion=$(SDKVERSION) \
                  $(ENABLE_FULL_RELRO_GOFLAGS)" \
                   -trimpath -mod=readonly
# ifeq ($(ENABLE_PIE), true)
# 	GOFLAGS += -buildmode=pie
# endif

build: $(MICROSERVICES)

cmd/device-wiresink:
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=0 \
	go build -tags "$(ADD_BUILD_TAGS)" $(GOFLAGS) -o $@ ./cmd

clean:
	rm -f $(MICROSERVICES)

docker: $(DOCKERS)

docker_device_wiresink_go: build
	docker build \
		--build-arg ADD_BUILD_TAGS=$(ADD_BUILD_TAGS) \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-wiresink:$(GIT_SHA) \
		-t edgexfoundry/device-wiresink:$(VERSION)-dev \
		.

docker_device_wiresink_go_arm64:
	$(MAKE) ARCH=aarch64 build
	docker buildx build --platform linux/arm64 \
		--build-arg ADD_BUILD_TAGS=$(ADD_BUILD_TAGS) \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-wiresink:$(GIT_SHA)-arm64 \
		-t edgexfoundry/device-wiresink:$(VERSION)-dev-arm64 \
		--load .