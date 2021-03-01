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

MODULE:=$(shell go mod edit -json | jq -r .Module.Path)
STAGING_DIR:=staging
CUSTOM_RESOURCES_DIR:=crd/crs

define generate
	echo "generation for $1/$2";\
	INPUT_DIRS=$(MODULE)/crd/models/$1/$2; \
	OUTPUT_PACKAGE=$(MODULE)/$(CUSTOM_RESOURCES_DIR)/$1/$2;\
	echo "Generating deep copy function ..."; \
	deepcopy-gen --go-header-file assets/boilerplate.go.txt --input-dirs $$INPUT_DIRS  --output-base $(STAGING_DIR); \
	echo "Generating clientset ..."; \
	client-gen --clientset-name versioned --input-base . --input $$INPUT_DIRS --output-package $$OUTPUT_PACKAGE/clientset --output-base $(STAGING_DIR) --go-header-file assets/boilerplate.go.txt; \
	echo "Generating listers ..."; \
	lister-gen --input-dirs $$INPUT_DIRS --output-package $$OUTPUT_PACKAGE/listers  --output-base $(STAGING_DIR)  --go-header-file assets/boilerplate.go.txt; \
	echo "Generating informers ..."; \
	informer-gen --input-dirs $$INPUT_DIRS --versioned-clientset-package $$OUTPUT_PACKAGE/clientset/versioned --output-base $(STAGING_DIR) --listers-package $$OUTPUT_PACKAGE/listers --output-package $$OUTPUT_PACKAGE/informers  --go-header-file assets/boilerplate.go.txt; \
	echo "Generating register ..."; \
	register-gen --input-dirs $$INPUT_DIRS --output-package $(MODULE)/models/$1/$2 --output-base $(STAGING_DIR) --go-header-file assets/boilerplate.go.txt; \
	echo "Generating defaults ..."; \
	defaulter-gen --go-header-file assets/boilerplate.go.txt --input-dirs $$INPUT_DIRS  --output-base $(STAGING_DIR); \
	cp -R $(STAGING_DIR)/$(MODULE)/crd/models/$1 ./crd/models;\
	cp -R $(STAGING_DIR)/$(MODULE)/crd/crs/$1 ./crd/crs;\
	rm -rf $(STAGING_DIR);
endef

.PHONY: crd_generate_all
crd_generate_all: crds=$(shell ls crd/models)
crd_generate_all:
	$(foreach crd,$(crds), \
		$(foreach version, $(shell ls crd/models/$(crd)),\
		$(call generate,$(crd),$(version))))

.PHONY: crd_generate
crd_generate: version?=v1
crd_generate:
ifndef crd
	$(error usage: make crd_generate crd=<crd name> [version=<crd version>])
endif
	$(call generate,$(crd),$(version))

.PHONY: crd_install_generators
crd_install_generators:
	git clone --depth 1 https://github.com/kubernetes/code-generator generators; \
	cd generators; go install ./cmd/{client-gen,lister-gen,informer-gen,deepcopy-gen,register-gen}; \
	cd ..; rm -rf generators;
