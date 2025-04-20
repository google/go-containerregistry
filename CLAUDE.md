# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands
- Run all tests: `go test ./...`
- Run specific test: `go test ./path/to/package -run TestName`
- Run linter: `staticcheck ./pkg/...`
- Verify formatting: `gofmt -d -e -l ./`
- Full presubmit checks: `./hack/presubmit.sh`
- Update generated docs: `./hack/update-codegen.sh`

## Code Style
- Use standard Go formatting (gofmt)
- Import ordering: standard library, third-party, internal packages
- Naming: follow Go conventions (CamelCase for exported, camelCase for unexported)
- Error handling: return errors with context rather than logging
- Testing: use standard Go testing patterns, include table-driven tests
- Copyright header required at top of each file
- Interface-driven design, with immutable view resources
- Implement minimal subset interfaces, use partial package for derived accessors
- Avoid binary data in logs (use redact.NewContext)
