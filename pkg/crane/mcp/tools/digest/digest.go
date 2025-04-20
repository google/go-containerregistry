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

// Package digest provides an MCP tool for getting the digest of an image.
package digest

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/crane/mcp/auth"
	"github.com/google/go-containerregistry/pkg/name"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// NewTool creates a new digest tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("digest",
		mcppkg.WithDescription("Get the digest of a container image"),
		mcppkg.WithString("image",
			mcppkg.Required(),
			mcppkg.Description("The image reference to get the digest for"),
		),
		mcppkg.WithBoolean("full-ref",
			mcppkg.Description("If true, print the full image reference with digest"),
		),
	)
}

// Handle handles digest tool requests.
func Handle(ctx context.Context, request mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	// Check if the required image parameter is present
	imageVal, ok := request.Params.Arguments["image"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: image"), nil
	}

	// Check if the image parameter is a string
	image, ok := imageVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter image: expected string"), nil
	}

	// Get the optional full-ref parameter
	fullRef, _ := request.Params.Arguments["full-ref"].(bool)

	// Get digest with authentication
	options := auth.CreateOptions(ctx)
	digest, err := crane.Digest(image, options...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error getting digest: %v", err)), nil
	}

	if fullRef {
		ref, err := name.ParseReference(image)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error parsing reference: %v", err)), nil
		}
		return mcppkg.NewToolResultText(ref.Context().Digest(digest).String()), nil
	}

	return mcppkg.NewToolResultText(digest), nil
}
