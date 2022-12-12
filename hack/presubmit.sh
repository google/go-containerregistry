#!/bin/bash

# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# We can't install in the current directory without changing the current module.
TMP_DIR="$(mktemp -d)"
export PATH="${PATH}:${TMP_DIR}/bin"
export GOPATH="${TMP_DIR}"
pushd ${TMP_DIR}
trap popd EXIT
go install honnef.co/go/tools/cmd/staticcheck@v0.3.3
popd

pushd ${PROJECT_ROOT}
trap popd EXIT

staticcheck ./pkg/...

# Verify that all source files are correctly formatted.
find . -name "*.go" | grep -v vendor/ | xargs gofmt -d -e -l

# Verify that generated crane docs are up-to-date.
mkdir -p /tmp/gendoc && go run cmd/crane/help/main.go --dir /tmp/gendoc && diff -Naur /tmp/gendoc/ cmd/crane/doc/

go test ./...
./pkg/name/internal/must_test.sh

./cmd/crane/rebase_test.sh

pushd ${PROJECT_ROOT}/cmd/krane
trap popd EXIT
go build ./...

pushd ${PROJECT_ROOT}/pkg/authn/k8schain
trap popd EXIT
go build ./...

pushd ${PROJECT_ROOT}/pkg/authn/kubernetes
trap popd EXIT
go test ./...
