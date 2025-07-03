#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

patches=(
  https://github.com/zregvart/go-containerregistry.git
  pr/credential-lookup
  https://github.com/google/go-containerregistry/pull/1966

  https://github.com/lcarva/go-containerregistry.git
  ignore-malformed-secrets
  https://github.com/google/go-containerregistry/pull/1834
)

for ((i = 0; i < ${#patches[@]}; i += 3)); do
  url="${patches[i]}"
  branch="${patches[i+1]}"
  upstream_pr="${patches[i+2]}"

  remote="$(tmp="${url%/*}"; echo "${tmp##*/}")"

  git remote add --no-fetch "${remote}" "${url}" || true # ignore if it already exists
  git fetch "${remote}" "${branch}"
  git merge \
    -m "Merge ${remote}/${branch}" \
    -m "Used here: https://github.com/conforma/cli/blob/main/go.mod#L61" \
    -m "See also: ${upstream_pr}" \
    "${remote}/${branch}"
done
