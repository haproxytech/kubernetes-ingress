#!/bin/sh
BIN_DIR="/tmp/check-commit/v$CHECK_COMMIT"

if [ -x "$BIN_DIR/check-commit" ]; then
    V=$("$BIN_DIR/check-commit" tag)
    if echo "$V" | grep -q "v$CHECK_COMMIT"; then
        echo "$V"
        exit 0
    fi
fi

mkdir -p "$BIN_DIR"
echo "go install github.com/haproxytech/check-commit/v5@v$CHECK_COMMIT"
GOBIN="$BIN_DIR" go install github.com/haproxytech/check-commit/v5@v$CHECK_COMMIT
