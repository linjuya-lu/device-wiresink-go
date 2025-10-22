ARG BASE=golang:1.23-alpine3.20
FROM ${BASE} AS builder

# 允许代理从外部传入
ARG http_proxy
ARG https_proxy
ARG no_proxy

ARG ALPINE_PKG_BASE="make git openssh-client"
ARG ALPINE_PKG_EXTRA=""
ARG ADD_BUILD_TAGS=""
ARG MAKE="make -e ADD_BUILD_TAGS=${ADD_BUILD_TAGS} build"

ARG APP_DIR=/device-wiresink-go
WORKDIR ${APP_DIR}

RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

# 拷贝 go.mod/vendor
COPY go.mod vendor* ./
RUN [ ! -d "vendor" ] && go mod download all || echo "skipping..."

COPY . .

# 构建 
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    set -eux; \
    ${MAKE}; \
    mkdir -p /out; \
    if [ -f cmd/device-wiresink ]; then \
        cp cmd/device-wiresink /out/device-wiresink; \
    elif [ -f cmd/device-wiresink-go ]; then \
        cp cmd/device-wiresink-go /out/device-wiresink; \
    else \
        bin="$(find cmd -maxdepth 1 -type f -perm -111 | head -n1)"; \
        cp "$bin" /out/device-wiresink; \
    fi; \
    for f in LICENSE Attribution.txt; do \
        [ -f "$f" ] && cp "$f" /out/ || true; \
    done

FROM alpine:3.20
# 把本地的 cmd/res 烘进镜像
ARG APP_DIR=/device-wiresink-go
COPY --from=builder ${APP_DIR}/cmd/res/ /res/
LABEL license='SPDX-License-Identifier: Apache-2.0' \
      copyright='Copyright (c) 2019-2021: IOTech'

# 最小化运行时：dumb-init 做 1 号进程，避免僵尸进程
RUN apk add --no-cache dumb-init && apk --no-cache upgrade

# 把 /out 里打包好的内容整体复制进来（不存在的单个文件不会导致失败）
COPY --from=builder /out/ /

EXPOSE 59910

# 用 dumb-init 作为 entrypoint 更稳
ENTRYPOINT ["/usr/bin/dumb-init", "--", "/device-wiresink"]
CMD ["-cp=keeper.http://edgex-core-keeper:59890", "--registry"]
