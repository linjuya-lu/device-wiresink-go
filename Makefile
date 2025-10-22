.PHONY: build test unittest lint clean docker

# change the following boolean flag to enable or disable the Full RELRO (RELocation Read Only) for linux ELF (Executable and Linkable Format) binaries
ENABLE_FULL_RELRO=true
# change the following boolean flag to enable or disable PIE for linux binaries which is needed for ASLR (Address Space Layout Randomization) on Linux, the ASLR support on Windows is enabled by default
ENABLE_PIE=true

MICROSERVICES=cmd/device-wiresink
.PHONY: $(MICROSERVICES)

# ========= 架构配置（支持命令行覆盖：make ARCH=arm64） =========
ARCH ?= aarch64

# 规范化 ARCH -> GOOS/GOARCH/GOARM
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

VERSION=$(shell cat ./VERSION 2>/dev/null || echo 0.0.0)
GIT_SHA=$(shell git rev-parse HEAD)
SDKVERSION=$(shell cat ./go.mod | grep 'github.com/edgexfoundry/device-sdk-go/v4 v' | sed 's/require//g' | awk '{print $$2}')

ifeq ($(ENABLE_FULL_RELRO), true)
	ENABLE_FULL_RELRO_GOFLAGS = -bindnow
	# NOTE: -bindnow 仅对使用外部链接器时生效；当前 CGO=0 走内部链接器，等需要 FULL RELRO 再改用 -extldflags。
endif

GOFLAGS=-ldflags "-s -w -X github.com/edgexfoundry/device-wiresink-go.Version=$(VERSION) \
                  -X github.com/edgexfoundry/device-sdk-go/v4/internal/common.SDKVersion=$(SDKVERSION) \
                  $(ENABLE_FULL_RELRO_GOFLAGS)" \
                   -trimpath -mod=readonly

ifeq ($(ENABLE_PIE), true)
	GOFLAGS += -buildmode=pie
endif

build: $(MICROSERVICES)

build-nats:
	$(MAKE) -e ADD_BUILD_TAGS=include_nats_messaging build

build-noziti:
	$(MAKE) -e ADD_BUILD_TAGS=no_openziti build

tidy:
	go mod tidy

cmd/device-wiresink:
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=0 \
	go build -tags "$(ADD_BUILD_TAGS)" $(GOFLAGS) -o $@ ./cmd


clean:
	rm -f $(MICROSERVICES)

docker: $(DOCKERS)

docker_device_wiresink_go:
	docker build \
		--build-arg ADD_BUILD_TAGS=$(ADD_BUILD_TAGS) \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-wiresink:$(GIT_SHA) \
		-t edgexfoundry/device-wiresink:$(VERSION)-dev \
		.

# arm64 镜像：在 x86 主机用 buildx 也能出 ARM64
docker_device_wiresink_go_arm64:
	docker buildx build --platform linux/arm64 \
		--build-arg ADD_BUILD_TAGS=$(ADD_BUILD_TAGS) \
		--label "git_sha=$(GIT_SHA)" \
		-t edgexfoundry/device-wiresink:$(GIT_SHA)-arm64 \
		-t edgexfoundry/device-wiresink:$(VERSION)-dev-arm64 \
		--load .

vendor:
	CGO_ENABLED=0 go mod vendor

# 打印当前目标架构
print-arch:
	@echo "ARCH=$(ARCH)  ->  GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM)"
