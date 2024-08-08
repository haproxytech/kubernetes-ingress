#!/bin/sh
V=$(./check-commit tag)

if echo "$V" | grep -q "v$CHECK_COMMIT"; then
    echo "$V"
else
    echo "go install github.com/haproxytech/check-commit/v5@v$CHECK_COMMIT"
    GOBIN=$(pwd) go install github.com/haproxytech/check-commit/v5@v$CHECK_COMMIT
fi
