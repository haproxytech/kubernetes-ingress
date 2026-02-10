PROJECT_PATH=${PWD}
TARGETPLATFORM?=linux/amd64
GOOS?=linux
GOARCH?=amd64
CHECK_COMMIT=5.4.0

.PHONY: test
test:
	gotestsum --format-icons=hivis --format=testdox --format-hide-empty-pkg -- $(go list ./... | grep -v /deploy/tests/e2e)

.PHONY: e2e
e2e:
	go clean -testcache
	go test ./... --tags=e2e_parallel,e2e_https
	go test ./... -p 1 --tags=e2e_sequential

.PHONY: e2e_crd_v1
e2e_crd_v1:
	go clean -testcache
	CRD_VERSION=v1 go test ./... --tags=e2e_parallel,e2e_https
	CRD_VERSION=v1 go test ./... -p 1 --tags=e2e_sequential


.PHONY: tidy
tidy:
	go mod tidy

.PHONY: doc
doc:
	cd cmd/docs; go run .

.PHONY: lint
lint:
	go install github.com/go-task/task/v3/cmd/task@latest
	task lint

.PHONY: check-commit
check-commit:
	CHECK_COMMIT=${CHECK_COMMIT} sh bin/check-commit.sh
	/tmp/check-commit/v${CHECK_COMMIT}/check-commit

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
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED='0' go build .
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
