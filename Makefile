PROJECT_PATH=${PWD}
TARGETPLATFORM?=linux/amd64
GOOS?=linux
GOARCH?=amd64
GOLANGCI_LINT_VERSION=1.64.5
CHECK_COMMIT=5.2.0

.PHONY: test
test:
	go test ./...

.PHONY: e2e
e2e:
	go clean -testcache
	go test ./... --tags=e2e_parallel,e2e_https
	go test ./... -p 1 --tags=e2e_sequential

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: doc
doc:
	cd documentation/gen/; go run .

.PHONY: lint
lint:
	cd bin;GOLANGCI_LINT_VERSION=${GOLANGCI_LINT_VERSION} sh lint-check.sh
	bin/golangci-lint run --timeout 20m --color always --max-issues-per-linter 0 --max-same-issues 0

.PHONY: lint-seq
lint-seq:
	cd bin;GOLANGCI_LINT_VERSION=${GOLANGCI_LINT_VERSION} sh lint-check.sh
	go run cmd/linters/*

.PHONY: check-commit
check-commit:
	cd bin;CHECK_COMMIT=${CHECK_COMMIT} sh check-commit.sh
	bin/check-commit

.PHONY: yaml-lint
yaml-lint:
	docker run --rm -v $(pwd):/data cytopia/yamllint .

.PHONY: example
example:
	deploy/tests/create.sh

.PHONY: example-pebble
example-pebble:
	CUSTOMDOCKERFILE=build/Dockerfile.pebble deploy/tests/create.sh

## Install the `example` with an image built from a local build.
.PHONY: example-dev
example-dev: build-dev
	CUSTOMDOCKERFILE=build/Dockerfile.dev deploy/tests/create.sh

.PHONY: example-experimental-gwapi
example-experimental-gwapi:
	EXPERIMENTAL_GWAPI=1 deploy/tests/create.sh

.PHONY: example-rebuild
example-rebuild:
	deploy/tests/rebuild.sh

.PHONY: example-remove
example-remove:
	deploy/tests/delete.sh

.PHONY: build
build:
	docker build -t haproxytech/kubernetes-ingress --build-arg TARGETPLATFORM=$(TARGETPLATFORM) -f build/Dockerfile .

.PHONY: build-pebble
build-pebble:
	docker build -t haproxytech/kubernetes-ingress --build-arg TARGETPLATFORM=$(TARGETPLATFORM) -f build/Dockerfile.pebble .

### build-dev builds locally an ingress-controller binary and copies it into the docker image.
### Can be used for example to use `go replace` and build with a local library,
.PHONY: build-dev
build-dev:
	GOOS=$(GOSS) GOARCH=$(GOARCH) CGO_ENABLED='0' go build .
	docker build -t haproxytech/kubernetes-ingress --build-arg TARGETPLATFORM=$(TARGETPLATFORM) -f build/Dockerfile.dev .

.PHONY: publish
publish:
	goreleaser release --rm-dist

.PHONY: cr_generate
cr_generate:
	crs/code-generator.sh

.PHONY: gofumpt
gofumpt:
	go install mvdan.cc/gofumpt@latest
	gofumpt -l -w .
