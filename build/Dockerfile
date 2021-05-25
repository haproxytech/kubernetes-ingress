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

FROM golang:1.16-buster AS builder

# RUN apk --no-cache add git openssh

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
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags "-X main.GitRepo=$(cat /var/run/vars/GIT_REPO) -X main.GitTag=$(cat /var/run/vars/GIT_LAST_TAG) -X main.GitCommit=$(cat /var/run/vars/GIT_HEAD_COMMIT) -X main.GitDirty=$(cat /var/run/vars/GIT_MODIFIED) -X main.BuildTime=$(cat /var/run/vars/BUILD_DATE)" \
        -o fs/haproxy-ingress-controller .

FROM haproxytech/haproxy-debian:2.3

COPY /fs/start.sh /
COPY /fs/usr/local/etc/haproxy/haproxy.cfg /usr/local/etc/haproxy/
COPY --from=builder /src/fs/haproxy-ingress-controller .

ARG DUMB_INIT_SHA256=cd7ab5513d20f4b985012d264fbdee60ccd2ea528b94b4b9308c6c6d26f8ba90

RUN apt-get update && \
    apt-get install -y socat openssl htop tzdata wget && \
    wget --no-check-certificate -O /dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.4/dumb-init_1.2.4_x86_64 && \
    rm -f /usr/local/bin/dataplaneapi /usr/bin/dataplaneapi && \
    echo "$DUMB_INIT_SHA256  /dumb-init" | sha256sum -c - && \
    chmod +x /dumb-init &&\
    chgrp -R haproxy /usr/local/etc/haproxy /run /var && \
    chmod -R g+w /usr/local/etc/haproxy /run /var

ENTRYPOINT ["/dumb-init", "--", "/start.sh"]
