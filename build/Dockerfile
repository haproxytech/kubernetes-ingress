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

FROM golang:1.19-alpine AS builder

RUN apk --no-cache add git openssh

COPY /go.mod /src/go.mod
COPY /go.sum /src/go.sum
RUN cd /src && go mod download

COPY / /src

RUN mkdir -p /var/run/vars && \
    cd /src && \
    git config --get remote.origin.url > /var/run/vars/GIT_REPO && \
    git rev-parse --short HEAD > /var/run/vars/GIT_HEAD_COMMIT && \
    git describe --abbrev=0 --tags > /var/run/vars/GIT_LAST_TAG && \
    git rev-parse --short $(cat /var/run/vars/GIT_LAST_TAG) > /var/run/vars/GIT_TAG_COMMIT && \
    git diff $(cat /var/run/vars/GIT_HEAD_COMMIT) $(cat /var/run/vars/GIT_TAG_COMMIT) --quiet > /var/run/vars/GIT_MODIFIED1 || echo '.dev' > /var/run/vars/GIT_MODIFIED1 && \
    git diff --quiet > /var/run/vars/GIT_MODIFIED2 || echo '.dirty' > /var/run/vars/GIT_MODIFIED2 && \
    cat /var/run/vars/GIT_MODIFIED1 /var/run/vars/GIT_MODIFIED2 | tr -d '\n' > /var/run/vars/GIT_MODIFIED && \
    date '+%Y-%m-%dT%H:%M:%S' > /var/run/vars/BUILD_DATE && \
    CGO_ENABLED=0 go build \
        -ldflags "-X main.GitRepo=$(cat /var/run/vars/GIT_REPO) -X main.GitTag=$(cat /var/run/vars/GIT_LAST_TAG) -X main.GitCommit=$(cat /var/run/vars/GIT_HEAD_COMMIT) -X main.GitDirty=$(cat /var/run/vars/GIT_MODIFIED) -X main.BuildTime=$(cat /var/run/vars/BUILD_DATE)" \
        -o fs/haproxy-ingress-controller .

FROM haproxytech/haproxy-alpine:2.5

ARG TARGETPLATFORM

ARG S6_OVERLAY_VERSION=2.2.0.3
ENV S6_OVERLAY_VERSION $S6_OVERLAY_VERSION
ENV S6_READ_ONLY_ROOT=1

COPY /fs /

RUN apk --no-cache add socat openssl util-linux htop tzdata curl libcap && \
    rm -f /usr/local/bin/dataplaneapi /usr/bin/dataplaneapi && \
    chgrp -R haproxy /usr/local/etc/haproxy /run /var && \
    chmod -R ug+rwx /usr/local/etc/haproxy /run /var && \
    setcap 'cap_net_bind_service=+ep' /usr/local/sbin/haproxy && \
    case "${TARGETPLATFORM}" in \
        "linux/arm64")      S6_ARCH=aarch64     ;; \
        "linux/amd64")      S6_ARCH=amd64       ;; \
        "linux/arm/v6")     S6_ARCH=arm         ;; \
        "linux/arm/v7")     S6_ARCH=armhf       ;; \
        "linux/ppc64le")    S6_ARCH=ppc64le     ;; \
        "linux/386")        S6_ARCH=x86         ;; \
        *) echo "ARG TARGETPLATFORM undeclared" >&2 && exit 1 ;; \
    esac && \
    curl -sS -L -o /tmp/s6-overlay-installer "https://github.com/just-containers/s6-overlay/releases/download/v${S6_OVERLAY_VERSION}/s6-overlay-${S6_ARCH}-installer" && \
    chmod +x /tmp/s6-overlay-installer && \
    /tmp/s6-overlay-installer / && \
    rm -f /tmp/s6-overlay-installer && \
    mkdir /var/run/s6 && \
    chown haproxy:haproxy /var/run/s6 && \
    chmod ug+rwx /var/run/s6 && \
    sed -i 's/ root / haproxy /g' /etc/s6/init/init-stage2-fixattrs.txt && \
    chmod ugo+x /etc/services.d/*/run /etc/cont-init.d/*

COPY --from=builder /src/fs/haproxy-ingress-controller .

ENTRYPOINT ["/start.sh"]
