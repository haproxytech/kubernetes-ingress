#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# set client native version
client_native_version=$(go list -m -f "{{.Version}}" github.com/haproxytech/client-native/v5)
echo "Client Native Version: $client_native_version"
for file in crs/api/ingress/v1/*.go; do
    echo "$file"
    # Use sed to replace the version string in Go files with the new version
    sed -i  "s@// +kubebuilder:metadata:annotations=\"haproxy.org/client-native=.*\"@// +kubebuilder:metadata:annotations=\"haproxy.org/client-native=$client_native_version\"@" $file
done

# code-generator build native, versioned clients, informers and other helpers
# via Kubernetes code generators from k8s.oi/code-generator

CR_DIR=$(dirname "$0")
OUTPUT_DIR="${CR_DIR}/.generated"
HDR_FILE="${CR_DIR}/../assets/license-header.txt"
CR_PKG="github.com/haproxytech/kubernetes-ingress/crs"
API_PKGS=$(find ${CR_DIR}/api -mindepth 2 -type d -printf "$CR_PKG/api/%P\n"| sort | tr '\n' ',')
API_PKGS=${API_PKGS::-1} # remove trailing ","

# Install Kubernetes Code Generators from k8s.io/code-generator

VERSION=$(go list -m  k8s.io/api  | cut -d ' ' -f2)
GOBIN="$(go env GOBIN)"
gopath="$(go env GOPATH)"
gobin="${GOBIN:-$(go env GOPATH)/bin}"
# new version is completly broken (with breaking changes \o/) use old one
#go install k8s.io/code-generator/cmd/{deepcopy-gen,register-gen,client-gen,lister-gen,informer-gen,defaulter-gen}@$VERSION
go install k8s.io/code-generator/cmd/{deepcopy-gen,register-gen,client-gen,lister-gen,informer-gen,defaulter-gen}@v0.29.5

# Generate Code
echo "Generating code for $API_PKGS"

echo "Generating deepcopy funcs"
GOPATH=$gopath "${gobin}/deepcopy-gen"\
  -O zz_generated.deepcopy\
	--input-dirs ${API_PKGS}\
	--go-header-file ${HDR_FILE}\
	--output-base ${OUTPUT_DIR}

echo "Generating register funcs"
GOPATH=$gopath "${gobin}/register-gen"\
  -O zz_generated.register\
	--input-dirs ${API_PKGS}\
	--go-header-file ${HDR_FILE}\
	--output-base ${OUTPUT_DIR}

echo "Generating clientset"
GOPATH=$gopath "${gobin}/client-gen"\
	--plural-exceptions "Defaults:Defaults"\
	--clientset-name "versioned"\
	--input ${API_PKGS}\
	--input-base "" \
	--output-package "${CR_PKG}/generated/clientset"\
	--go-header-file ${HDR_FILE}\
	--output-base ${OUTPUT_DIR}

echo "Generating listers"
GOPATH=$gopath "${gobin}/lister-gen"\
	--plural-exceptions "Defaults:Defaults"\
	--input-dirs ${API_PKGS}\
	--output-package "${CR_PKG}/generated/listers"\
	--go-header-file ${HDR_FILE}\
	--output-base ${OUTPUT_DIR}

echo "Generating informers"
GOPATH=$gopath "${gobin}/informer-gen"\
	--plural-exceptions "Defaults:Defaults"\
	--input-dirs ${API_PKGS}\
	--versioned-clientset-package "${CR_PKG}/generated/clientset/versioned"\
	--listers-package "${CR_PKG}/generated/listers"\
	--output-package "${CR_PKG}/generated/informers"\
	--go-header-file ${HDR_FILE}\
	--output-base ${OUTPUT_DIR}

# Copy generated code into the right location
# This extra step is required because code generator seems to be working only in a GOPATH environment.
# https://github.com/kubernetes/code-generator/issues/57
# So the work around is to generate to OUTPUT_DIR then move code to the right location
find  ${OUTPUT_DIR}/${CR_PKG}/api -mindepth 2 -type f -printf "%P\n" | xargs -I{} cp ${OUTPUT_DIR}/${CR_PKG}/api/{} ${CR_DIR}/api/{}
mkdir -p ${CR_DIR}/generated
cp -r ${OUTPUT_DIR}/${CR_PKG}/generated/{clientset,listers,informers} ${CR_DIR}/generated
rm -rf ${OUTPUT_DIR}

CONTROLLER_GEN_VERSION=$(go list -m  sigs.k8s.io/controller-tools  | cut -d ' ' -f2)
go install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}

# Controller-gen version
echo "Controller-gen: " ${CONTROLLER_GEN_VERSION}
controller-gen crd paths=./crs/api/ingress/...  output:crd:dir=./crs/definition
# remove code-gen annotation (dependabot fails)
find ${CR_DIR}/definition -type f -name '*.yaml' -exec sed -i '/controller-gen.kubebuilder.io\/version/d' {} +
