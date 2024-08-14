#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

patches=(https://github.com/zregvart/go-containerregistry.git pr/credential-lookup https://github.com/lcarva/go-containerregistry.git ignore-malformed-secrets)

for ((i = 0; i < ${#patches[@]}; i += 2)); do
  url="${patches[i]}"
  branch="${patches[i+1]}"
  remote="$(tmp="${url%/*}"; echo "${tmp##*/}")"
  git remote add --no-fetch "${remote}" "${url}" || true # ignore if it already exists
  git fetch "${remote}" "${branch}"
  git merge -m "Merge ${url}/${branch}" "${remote}"/"${branch}"
done
