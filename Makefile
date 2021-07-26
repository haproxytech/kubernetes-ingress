PROJECT_PATH=${PWD}

.PHONY: test
test:
	go test ./...

.PHONY: e2e
e2e:
	go test ./... --tags=e2e_parallel
	go test ./... -p 1 --tags=e2e_sequential

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: doc
doc:
	cd documentation/gen/; go run .

.PHONY: lint
lint:
	docker run --rm -v $(pwd):/data cytopia/yamllint .
	golangci-lint run --color always --timeout 240s

.PHONY: example
example:
	deploy/tests/create.sh

.PHONY: example-rebuild
example-rebuild:
	deploy/tests/rebuild.sh

.PHONY: example-remove
example-remove:
	deploy/tests/delete.sh

.PHONY: build
build:
	docker build -t haproxytech/kubernetes-ingress -f build/Dockerfile .

.PHONY: publish
publish:
	goreleaser release --rm-dist
