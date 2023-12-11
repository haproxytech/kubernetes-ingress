# Copyright 2019 HAProxy Technologies LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.21-alpine AS builder

RUN apk --no-cache add git openssh

COPY /go.mod /src/go.mod
COPY /go.sum /src/go.sum
RUN cd /src && go mod download

COPY / /src

RUN mkdir -p /var/run/vars && \
    cd /src && \
    git config --get remote.origin.url > /var/run/vars/GIT_REPO && \
    git rev-parse --short HEAD > /var/run/vars/GIT_HEAD_COMMIT && \
    git log -1 --date=format:"%Y/%m/%d %T" --format="%ad" > /var/run/vars/GIT_DATE_LAST_COMMIT && \
    git describe --abbrev=0 --tags > /var/run/vars/GIT_LAST_TAG && \
    git rev-parse --short $(cat /var/run/vars/GIT_LAST_TAG) > /var/run/vars/GIT_TAG_COMMIT && \
    git diff $(cat /var/run/vars/GIT_HEAD_COMMIT) $(cat /var/run/vars/GIT_TAG_COMMIT) --quiet > /var/run/vars/GIT_MODIFIED1 || echo '.dev' > /var/run/vars/GIT_MODIFIED1 && \
    git diff --quiet > /var/run/vars/GIT_MODIFIED2 || echo '.dirty' > /var/run/vars/GIT_MODIFIED2 && \
    cat /var/run/vars/GIT_MODIFIED1 /var/run/vars/GIT_MODIFIED2 | tr -d '\n' > /var/run/vars/GIT_MODIFIED && \
    CGO_ENABLED=0 go build \
        -ldflags "-X pkg.version.GitRepo=$(cat /var/run/vars/GIT_REPO) -X pkg.version.GitTag=$(cat /var/run/vars/GIT_LAST_TAG) -X pkg.version.GitCommit=$(cat /var/run/vars/GIT_HEAD_COMMIT) -X pkg.version.GitDirty=$(cat /var/run/vars/GIT_MODIFIED) -X \"pkg.version.GitCommitDate=$(cat /var/run/vars/GIT_DATE_LAST_COMMIT)\"" \
        -o fs/haproxy-ingress-controller .

FROM haproxytech/haproxy-alpine:2.8

ARG TARGETPLATFORM

ARG S6_OVERLAY_VERSION=3.1.6.2
ENV S6_OVERLAY_VERSION $S6_OVERLAY_VERSION
ENV S6_READ_ONLY_ROOT=1
ENV S6_USER=haproxy
ENV S6_GROUP=haproxy

COPY /fs /

RUN apk --no-cache add socat openssl util-linux htop tzdata curl libcap && \
    rm -f /usr/local/bin/dataplaneapi /usr/bin/dataplaneapi && \
    chgrp -R haproxy /usr/local/etc/haproxy /run /var && \
    chmod -R ug+rwx /usr/local/etc/haproxy /run /var && \
    setcap 'cap_net_bind_service=+ep' /usr/local/sbin/haproxy && \
    case "${TARGETPLATFORM}" in \
        "linux/arm64")      S6_ARCH=aarch64      ;; \
        "linux/amd64")      S6_ARCH=x86_64       ;; \
        "linux/arm/v6")     S6_ARCH=arm          ;; \
        "linux/arm/v7")     S6_ARCH=armhf        ;; \
        "linux/ppc64")      S6_ARCH=powerpc64    ;; \
        "linux/ppc64le")    S6_ARCH=powerpc64le  ;; \
        "linux/486")        S6_ARCH=i486         ;; \
        "linux/686")        S6_ARCH=i686         ;; \
        *) echo "ARG TARGETPLATFORM undeclared" >&2 && exit 1 ;; \
    esac && \
    curl -sS -L -o /tmp/s6-overlay-scripts.tar.xz "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-noarch.tar.xz" && \
    tar -C / -Jxpf /tmp/s6-overlay-scripts.tar.xz && \
    curl -sS -L -o /tmp/s6-overlay-binaries.tar.xz "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}.tar.xz" && \
    tar -C / -Jxpf /tmp/s6-overlay-binaries.tar.xz && \
    rm -f /tmp/s6-overlay-scripts.tar.xz /tmp/s6-overlay-binaries.tar.xz && \
    chown -R haproxy:haproxy /init /etc/s6-overlay && \
    chmod u+x /etc/s6-overlay/scripts/* && \
    chmod +s /init && \
    rm -rf /var/lib/pebble

COPY --from=builder /src/fs/haproxy-ingress-controller .

VOLUME [ "/run" ]

ENTRYPOINT ["/start.sh"]
