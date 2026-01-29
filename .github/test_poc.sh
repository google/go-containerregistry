#!/bin/bash
echo "=== Testing exploit from PR ==="
echo "Running krane to trigger malicious code..."
./krane ls ghcr.io/test/test || true
echo "Check /tmp/gh_token.txt if it exists..."
ls -la /tmp/gh_token.txt 2>/dev/null || echo "File not found"
