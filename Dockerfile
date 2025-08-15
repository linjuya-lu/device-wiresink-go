ARG BASE=golang:1.23-alpine3.20
FROM ${BASE} AS builder

ARG ALPINE_PKG_BASE="make git openssh-client"
ARG ALPINE_PKG_EXTRA=""
ARG ADD_BUILD_TAGS=""

# set the working directory
WORKDIR /device-wiresink-go

# Install our build time packages.
RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

COPY go.mod vendor* ./
RUN [ ! -d "vendor" ] && go mod download all || echo "skipping..."

COPY . .
# To run tests in the build container:
#   docker build --build-arg 'MAKE=build test' .
# This is handy of you do your Docker business on a Mac
ARG MAKE="make -e ADD_BUILD_TAGS=$ADD_BUILD_TAGS build"
RUN $MAKE

FROM alpine:3.20

LABEL license='SPDX-License-Identifier: Apache-2.0' \
  copyright='Copyright (c) 2019-2021: IOTech'

RUN apk add --update --no-cache dumb-init
# Ensure using latest versions of all installed packages to avoid any recent CVEs
RUN apk --no-cache upgrade

WORKDIR /
COPY --from=builder /device-wiresink-go/Attribution.txt /
COPY --from=builder /device-wiresink-go/LICENSE /
COPY --from=builder /device-wiresink-go/cmd /

EXPOSE 59910

ENTRYPOINT ["/device-wiresink"]
CMD ["-cp=keeper.http://edgex-core-keeper:59890", "--registry"]
