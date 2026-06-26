#!/bin/sh
# Copyright 2025 HAProxy Technologies LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
#
# Builds libblock_secrets.so and runs block_secrets_test.c under LD_PRELOAD.
# Designed to run inside the musl/alpine build image so the test exercises the
# same libc as production. Usage:
#   docker run --rm -v "$PWD/pkg/protection":/p:ro haproxytech/haproxy-alpine:3.2 \
#       sh -c 'apk add --no-cache build-base >/dev/null && cp /p/* /tmp && /tmp/block_secrets_test.sh'
set -eu

SRC_DIR="$(cd "$(dirname "$0")" && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cc -O2 -std=c11 -fPIC -shared -o "$WORK/libblock_secrets.so" \
    "$SRC_DIR/block_secrets.c" -ldl
cc -O2 -std=c11 -o "$WORK/block_secrets_test" "$SRC_DIR/block_secrets_test.c" -ldl

# Protected dir with a "token" inside, plus a normal file outside it.
BLOCKED_DIR="$WORK/secrets/kubernetes.io"
mkdir -p "$BLOCKED_DIR"
echo "secret-token" > "$BLOCKED_DIR/token"
echo "data" > "$WORK/normal.txt"

BLOCK_SECRETS_PATH="$BLOCKED_DIR" \
TEST_BLOCKED_FILE="$BLOCKED_DIR/token" \
TEST_NORMAL_FILE="$WORK/normal.txt" \
LD_PRELOAD="$WORK/libblock_secrets.so" \
    "$WORK/block_secrets_test"
