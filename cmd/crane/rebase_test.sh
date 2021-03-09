#!/bin/bash
set -ex

tmp=$(mktemp -d)

go install ./cmd/registry
go build -o ./crane ./cmd/crane

# Start a local registry.
registry &
PID=$!
function cleanup {
  kill $PID
  rm -r ${tmp}
  rm ./crane
}
trap cleanup EXIT

sleep 1  # Wait for registry to be up.

# Create an image localhost:1338/base containing a.txt
echo a > ${tmp}/a.txt
./crane append -f <(tar -f - -c ${tmp}) -t localhost:1338/base
rm ${tmp}/a.txt

# Append to that image localhost:1338/rebaseme
echo top > ${tmp}/top.txt
./crane append -f <(tar -f - -c ${tmp}) -b localhost:1338/base -t localhost:1338/rebaseme
rm ${tmp}/top.txt

# Annotate that image as the base image (by ref and digest)
# TODO: do this with a flag to --append
./crane mutate localhost:1338/rebaseme \
  --annotation org.opencontainers.image.base.name=localhost:1338/base \
  --annotation org.opencontainers.image.base.digest=$(./crane digest localhost:1338/base) \
  --tag localhost:1338/rebaseme

# Update localhost:1338/base containing b.txt
echo b > ${tmp}/b.txt
./crane append -f <(tar -f - -c ${tmp}) -t localhost:1338/base
rm ${tmp}/b.txt

# Rebase using annotations
rebased=$(./crane rebase localhost:1338/rebaseme)

# List files in the rebased image.
./crane export ${rebased} - | tar -tvf -

# Extract b.txt out of the rebased image
./crane export ${rebased} - | tar -Oxf - ${tmp:1}/b.txt

# Extract top.txt out of the rebased image
./crane export ${rebased} - | tar -Oxf - ${tmp:1}/top.txt

# a.txt is _not_ in the rebased image.
set +e
./crane export ${rebased} - | tar -Oxf - ${tmp:1}/a.txt  # this should fail
code=$?
echo "finding a.txt exited ${code}"
if [[ $code -ne 1 ]]; then
  echo "a.txt was found in rebased image"
  exit 1
fi

