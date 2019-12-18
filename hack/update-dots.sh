#!/bin/bash

# Copyright 2019 The original author or authors
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

dot -Tjpeg images/ociimage.gv > images/ociimage.jpeg

for f in $(ls images/dot/ | grep -e '.dot$'); do
  dot -Tsvg images/dot/$f > images/$f.svg
done
