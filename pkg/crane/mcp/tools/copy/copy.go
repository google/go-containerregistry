// Copyright 2025 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package copy provides an MCP tool for copying container images.
package copy

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/crane/mcp/auth"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// NewTool creates a new copy tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("copy",
		mcppkg.WithDescription("Copy an image from one registry to another"),
		mcppkg.WithString("source",
			mcppkg.Required(),
			mcppkg.Description("The source image reference"),
		),
		mcppkg.WithString("destination",
			mcppkg.Required(),
			mcppkg.Description("The destination image reference"),
		),
	)
}

// Handle handles copy tool requests.
func Handle(ctx context.Context, request mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	// Check if required parameters are present
	sourceVal, ok := request.Params.Arguments["source"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: source"), nil
	}
	source, ok := sourceVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter source: expected string"), nil
	}

	destVal, ok := request.Params.Arguments["destination"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: destination"), nil
	}
	destination, ok := destVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter destination: expected string"), nil
	}

	// Get options with authentication
	options := auth.CreateOptions(ctx)

	if err := crane.Copy(source, destination, options...); err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error copying image: %v", err)), nil
	}

	return mcppkg.NewToolResultText(fmt.Sprintf("Successfully copied %s to %s", source, destination)), nil
}
