#!/bin/bash

# Copyright 2022 Google LLC
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

pushd ${PROJECT_ROOT}
trap popd EXIT

go get -u ./...
go mod tidy -compat=1.18
go mod vendor

cd ${PROJECT_ROOT}/pkg/authn/k8schain
go get -u ./...
go mod tidy -compat=1.18
go mod download

cd ${PROJECT_ROOT}/pkg/authn/kubernetes
go get -u ./...
go mod tidy -compat=1.18
go mod download

cd ${PROJECT_ROOT}/cmd/krane
go get -u ./...
go mod tidy -compat=1.18
go mod download

cd ${PROJECT_ROOT}

./hack/update-deps.sh
