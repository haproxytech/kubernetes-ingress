#!/bin/sh
V=$(./golangci-lint --version)

case "$V" in
  *$GOLANGCI_LINT_VERSION*) echo "$V" ;;
  *)          curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(pwd) "v$GOLANGCI_LINT_VERSION" ;;
esac
