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
old_base=$(./crane append -f <(tar -f - -c ${tmp}) -t localhost:1338/base)
rm ${tmp}/a.txt

# Append to that image localhost:1338/rebaseme
echo top > ${tmp}/top.txt
orig=$(./crane append -f <(tar -f - -c ${tmp}) -b ${old_base} -t localhost:1338/rebaseme)
rm ${tmp}/top.txt

# Annotate that image as the base image (by ref and digest)
# TODO: do this with a flag to --append
orig=$(./crane mutate ${orig} \
  --annotation org.opencontainers.image.base.name=localhost:1338/base \
  --annotation org.opencontainers.image.base.digest=$(./crane digest localhost:1338/base))

# Update localhost:1338/base containing b.txt
echo b > ${tmp}/b.txt
new_base=$(./crane append -f <(tar -f - -c ${tmp}) -t localhost:1338/base)
rm ${tmp}/b.txt

# Rebase using annotations
rebased=$(./crane rebase ${orig})

# List files in the rebased image.
./crane export ${rebased} - | tar -tvf -

# Extract b.txt out of the rebased image.
./crane export ${rebased} - | tar -Oxf - ${tmp:1}/b.txt

# Extract top.txt out of the rebased image.
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


# Test #2: use rebase to append all layers of one image to another
set -ex

# Create an image localhost:1338/common containing two layers
echo first > ${tmp}/layer1.txt
common=$(./crane append -f <(tar -f - -c ${tmp}) -t localhost:1338/common)
rm ${tmp}/layer1.txt

echo second > ${tmp}/layer2.txt
common=$(./crane append -f <(tar -f - -c ${tmp}) -b ${common} -t localhost:1338/common)
rm ${tmp}/layer2.txt

# Create an image localhost:1338/target containing base.txt
echo base > ${tmp}/base.txt
target=$(./crane append -f <(tar -f - -c ${tmp}) -t localhost:1338/target)
rm ${tmp}/base.txt

# Use rebase to append all layers of common image to target image
merged=$(./crane rebase --old_base scratch --new_base ${target} ${common})

# List files in the rebased image.
./crane export ${merged} - | tar -tvf -

# Extract layer1.txt out of the rebased image.
./crane export ${merged} - | tar -Oxf - ${tmp:1}/layer1.txt

# Extract layer2.txt out of the rebased image.
./crane export ${merged} - | tar -Oxf - ${tmp:1}/layer2.txt

# Extract base.txt out of the rebased image.
./crane export ${merged} - | tar -Oxf - ${tmp:1}/base.txt

# Verified all layers present in merged image
