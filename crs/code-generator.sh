#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# set client native version
client_native_version=$(go list -m -f "{{.Version}}" github.com/haproxytech/client-native/v6)
echo "Client Native Version: $client_native_version"
for file in crs/api/ingress/v3/*.go; do
    echo "$file"
    # Use sed to replace the version string in Go files with the new version
    sed -i  "s@// +kubebuilder:metadata:annotations=\"haproxy.org/client-native=.*\"@// +kubebuilder:metadata:annotations=\"haproxy.org/client-native=$client_native_version\"@" $file
done

# code-generator build native, versioned clients, informers and other helpers
# via Kubernetes code generators from k8s.oi/code-generator

CR_DIR=$(dirname "$0")
HDR_FILE="${CR_DIR}/../assets/license-header.txt"
CR_PKG="github.com/haproxytech/kubernetes-ingress/crs"
API_PKGS=$(find ${CR_DIR}/api -mindepth 2 -type d -printf "$CR_PKG/api/%P\n"| sort | tr '\n' ',')
API_PKGS=${API_PKGS::-1} # remove trailing ","

# Install Kubernetes Code Generators from k8s.io/code-generator

VERSION=$(go list -m  k8s.io/api  | cut -d ' ' -f2)
gobin="${GOBIN:-$(go env GOPATH)/bin}"
go install k8s.io/code-generator/cmd/{deepcopy-gen,register-gen,client-gen,lister-gen,informer-gen}@$VERSION

# Generate Code
# Each API package writes to its own output directory, so the packages are
# generated concurrently. Their logs are buffered and printed per package,
# otherwise the two runs interleave into something unreadable.
generate_api_pkg() {
    API_PKG="$1"
    CR_VERSION=${API_PKG#"$CR_PKG/"}
    GEN_DIR="${CR_DIR}/generated/${CR_VERSION}"
    GEN_PKG="${CR_PKG}/generated/${CR_VERSION}"
    echo "Generating code for $API_PKG"

    echo "Generating deepcopy funcs"
    "${gobin}/deepcopy-gen"\
        --output-file zz_generated.deepcopy.go\
        --go-header-file ${HDR_FILE}\
        "${API_PKG}"

    echo "Generating register funcs"
    "${gobin}/register-gen"\
        --output-file zz_generated.register.go\
        --go-header-file ${HDR_FILE}\
        "${API_PKG}"

    echo "Generating clientset"
    rm -rf "${GEN_DIR}/clientset"
    "${gobin}/client-gen"\
        --plural-exceptions "Defaults:Defaults,ValidationRules:ValidationRules"\
        --clientset-name "versioned"\
        --input "${API_PKG}"\
        --input-base ""\
        --output-dir "${GEN_DIR}/clientset"\
        --output-pkg "${GEN_PKG}/clientset"\
        --go-header-file ${HDR_FILE}

    echo "Generating listers"
    rm -rf "${GEN_DIR}/listers"
    "${gobin}/lister-gen"\
        --plural-exceptions "Defaults:Defaults,ValidationRules:ValidationRules"\
        --output-dir "${GEN_DIR}/listers"\
        --output-pkg "${GEN_PKG}/listers"\
        --go-header-file ${HDR_FILE}\
        "${API_PKG}"

    echo "Generating informers"
    rm -rf "${GEN_DIR}/informers"
    "${gobin}/informer-gen"\
        --plural-exceptions "Defaults:Defaults,ValidationRules:ValidationRules"\
        --versioned-clientset-package "${GEN_PKG}/clientset/versioned"\
        --listers-package "${GEN_PKG}/listers"\
        --output-dir "${GEN_DIR}/informers"\
        --output-pkg "${GEN_PKG}/informers"\
        --go-header-file ${HDR_FILE}\
        "${API_PKG}"
}

LOG_DIR=$(mktemp -d)
pids=""
IFS=','
for API_PKG in $API_PKGS; do
    generate_api_pkg "$API_PKG" > "${LOG_DIR}/$(echo "$API_PKG" | tr '/' '_').log" 2>&1 &
    pids="$pids $!"
done

unset IFS
status=0
for pid in $pids; do
    wait "$pid" || status=1
done
for log in "${LOG_DIR}"/*.log; do
    cat "$log"
done
rm -rf "${LOG_DIR}"
if [ "$status" -ne 0 ]; then
    echo "code generation failed" >&2
    exit 1
fi
IFS=','

CONTROLLER_GEN_VERSION=$(go list -m  sigs.k8s.io/controller-tools  | cut -d ' ' -f2)
go install sigs.k8s.io/controller-tools/cmd/controller-gen@${CONTROLLER_GEN_VERSION}

# # Controller-gen version
echo "Controller-gen: " ${CONTROLLER_GEN_VERSION}
controller-gen crd paths=./crs/api/ingress/v3/...  output:crd:dir=./crs/definition
# remove code-gen annotation (dependabot fails)
find ${CR_DIR}/definition -type f -name '*.yaml' -exec sed -i '/controller-gen.kubebuilder.io\/version/d' {} +


# # Removal of some fields from the CRDs
# # For example, for now we remove servers from the backend CRD v3
echo "Removing fields from the generated CRDs"
sh crs/remove-fields-from-crds.sh
