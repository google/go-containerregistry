#!/bin/bash

set -e

export GO111MODULE=on

if [ -z "$1" ]; then
      echo "usage $0 v1.19.7"
      exit
fi

TAG="${1}"
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
K8S_CHAIN_ROOT="${PROJECT_ROOT}/pkg/authn/k8schain"
PKG_TARGET="${K8S_CHAIN_ROOT}/internal/credentialprovider"

pushd ${PROJECT_ROOT}
trap popd EXIT

mkdir -p "${PKG_TARGET}"

CLONE_DIR=$(mktemp -d)

git clone --depth 1 --branch "${TAG}" git@github.com:kubernetes/kubernetes.git "${CLONE_DIR}"
cp -r "${CLONE_DIR}/pkg/credentialprovider/." "${PKG_TARGET}/."

find "${PKG_TARGET}" \( \
     -name "OWNERS" \
  -o -name "OWNERS_ALIASES" \
  -o -name "BUILD" \
  -o -name "BUILD.bazel" \) -exec rm -f {} +


OLD_NAME="k8s.io/kubernetes/pkg/credentialprovider"
NEW_NAME="github.com/google/go-containerregistry/pkg/authn/k8schain/internal/credentialprovider"

find "${PKG_TARGET}" -type f \
  -exec sed -i '' "s,${OLD_NAME},${NEW_NAME},g" {} \;

MODULE_VERSION="v0.$(echo "${TAG}" | cut -d '.' -f 2-)"

pushd $K8S_CHAIN_ROOT
go mod edit -json | jq -r '.Require[].Path' | grep ^k8s.io | xargs -n1 -I {} go get -u {}@"${MODULE_VERSION}" || true
go test ./...
go mod tidy
goimports -w .
popd
