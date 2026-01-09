#!/bin/bash
set -e

# Define paths
CRANE_CMD="go run cmd/crane/main.go"
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "Using temp dir: $TEST_DIR"

# Pull small images to use as base
echo "Pulling base to tarball..."
$CRANE_CMD pull gcr.io/distroless/base "$TEST_DIR/alpine.tar"
echo "Pulling static to tarball..."
$CRANE_CMD pull gcr.io/distroless/static "$TEST_DIR/busybox.tar"

# Test 1: Append local tarballs to a new local index (directory)
echo "Test 1: Append local tarballs to new local index (directory)..."
$CRANE_CMD index append "$TEST_DIR/my-index" "$TEST_DIR/alpine.tar" "$TEST_DIR/busybox.tar"

if [ -d "$TEST_DIR/my-index" ]; then
    echo "PASS: Index directory created."
else
    echo "FAIL: Index directory not created."
    exit 1
fi

# Verify content with list
echo "Verifying content of my-index..."
LIST_OUTPUT=$($CRANE_CMD index list "$TEST_DIR/my-index")
echo "$LIST_OUTPUT"
if echo "$LIST_OUTPUT" | grep -q "alpine"; then
    # Note: list output shows digests, not names, unless we annotated them?
    # Actually list output shows Digest, MediaType, Platform.
    # We can check count.
    true
fi
COUNT=$(echo "$LIST_OUTPUT" | grep -c "sha256:")
if [ "$COUNT" -eq 2 ]; then
    echo "PASS: Index contains 2 manifests."
else
    echo "FAIL: Index contains $COUNT manifests (expected 2)."
    exit 1
fi

# Test 2: Append to a "tar file index" (directory named .tar)
# This verifies we can create/write to a path ending in .tar, treating it as a layout directory.
echo "Test 2: Append to 'tar file index' (directory named .tar)..."
$CRANE_CMD index append "$TEST_DIR/output.tar" "$TEST_DIR/alpine.tar"

if [ -d "$TEST_DIR/output.tar" ]; then
    echo "PASS: Index directory (named .tar) created."
else
    echo "FAIL: Index directory (named .tar) not created."
    exit 1
fi

# Verify content
echo "Verifying content of output.tar..."
LIST_OUTPUT=$($CRANE_CMD index list "$TEST_DIR/output.tar")
echo "$LIST_OUTPUT"
COUNT=$(echo "$LIST_OUTPUT" | grep -c "sha256:")
if [ "$COUNT" -eq 1 ]; then
    echo "PASS: Index contains 1 manifest."
else
    echo "FAIL: Index contains $COUNT manifests (expected 1)."
    exit 1
fi

# Test 3: Append to existing index
echo "Test 3: Append to existing index..."
$CRANE_CMD index append "$TEST_DIR/output.tar" "$TEST_DIR/busybox.tar"
LIST_OUTPUT=$($CRANE_CMD index list "$TEST_DIR/output.tar")
COUNT=$(echo "$LIST_OUTPUT" | grep -c "sha256:")
if [ "$COUNT" -eq 2 ]; then
    echo "PASS: Index now contains 2 manifests."
else
    echo "FAIL: Index contains $COUNT manifests (expected 2)."
    exit 1
fi

# Test 5: Negative test - Append to an existing file (not a layout directory)
echo "Test 5: Negative test - Append to existing file..."
# alpine.tar is a file (tarball), not an OCI layout directory
set +e
$CRANE_CMD index append "$TEST_DIR/alpine.tar" "$TEST_DIR/busybox.tar"
EXIT_CODE=$?
set -e

if [ $EXIT_CODE -ne 0 ]; then
    echo "PASS: Failed to append to a file (as expected)."
else
    echo "FAIL: Unexpectedly succeeded appending to a file."
    exit 1
fi



# Test 6: Append remote image to local index
echo "Test 6: Append remote image to local index..."
# gcr.io/distroless/base is an index with several manifests.
# flattened=true by default.
$CRANE_CMD index append "$TEST_DIR/my-index-remote" "$TEST_DIR/busybox.tar" gcr.io/distroless/base
LIST_OUTPUT=$($CRANE_CMD index list "$TEST_DIR/my-index-remote")
COUNT=$(echo "$LIST_OUTPUT" | grep -c "sha256:")
# 1 from busybox.tar + ~3-5 from distroless
if [ "$COUNT" -ge 3 ]; then
    echo "PASS: Index contains $COUNT manifests."
else
    echo "FAIL: Index contains $COUNT manifests (expected >= 3)."
    exit 1
fi

if [ -z "$CRANE_TEST_REPO" ]; then
    echo "Skipping remote tests. Set CRANE_TEST_REPO to run them."
else
    echo "Running remote tests against $CRANE_TEST_REPO..."
    REPO="$CRANE_TEST_REPO"
    TAG_1="$REPO:index-test-1"

    # Remote Test 1: Create new remote index from local tarball
    echo "Remote Test 1: Create new remote index $TAG_1 from local alpine.tar..."
    # Note: We need --tag because we are creating from scratch (or implicit empty base)
    $CRANE_CMD index append -m "$TEST_DIR/alpine.tar" -t "$TAG_1"
    
    # Verify
    LIST_OUTPUT=$($CRANE_CMD index list "$TAG_1")
    COUNT=$(echo "$LIST_OUTPUT" | grep -c "sha256:")
    if [ "$COUNT" -eq 1 ]; then
        echo "PASS: Remote index created with 1 manifest."
    else
        echo "FAIL: Remote index contains $COUNT manifests (expected 1)."
        exit 1
    fi

    # Remote Test 2: Append remote image to existing remote index
    echo "Remote Test 2: Append distroless (remote) to $TAG_1..."
    # We use TAG_1 as base, and update it in place (or we could use -t to same tag)
    # Using positional arg as base
    $CRANE_CMD index append "$TAG_1" -m "gcr.io/distroless/static:latest" -t "$TAG_1"
    
    # Verify
    LIST_OUTPUT=$($CRANE_CMD index list "$TAG_1")
    COUNT=$(echo "$LIST_OUTPUT" | grep -c "sha256:")
    if [ "$COUNT" -ge 4 ]; then
        echo "PASS: Remote index now contains $COUNT manifests."
    else
        echo "FAIL: Remote index contains $COUNT manifests (expected >= 4)."
        exit 1
    fi
fi

echo "All tests passed!"
