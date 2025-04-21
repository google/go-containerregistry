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

// Package pull provides an MCP tool for pulling container images.
package pull

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/crane/mcp/auth"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// NewTool creates a new pull tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("pull",
		mcppkg.WithDescription("Pull a container image and save it as a tarball"),
		mcppkg.WithString("image",
			mcppkg.Required(),
			mcppkg.Description("The image reference to pull"),
		),
		mcppkg.WithString("output",
			mcppkg.Required(),
			mcppkg.Description("Path where the image tarball will be saved"),
		),
		mcppkg.WithString("format",
			mcppkg.Description("Format in which to save the image (tarball, legacy, or oci)"),
			mcppkg.DefaultString("tarball"),
			mcppkg.Enum("tarball", "legacy", "oci"),
		),
	)
}

// Handle handles pull tool requests.
func Handle(ctx context.Context, request mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	// Check if required parameters are present
	imageVal, ok := request.Params.Arguments["image"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: image"), nil
	}
	image, ok := imageVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter image: expected string"), nil
	}

	outputVal, ok := request.Params.Arguments["output"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: output"), nil
	}
	output, ok := outputVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter output: expected string"), nil
	}

	// Format has a default value
	format := "tarball" // Default value
	if formatVal, ok := request.Params.Arguments["format"]; ok {
		formatStr, ok := formatVal.(string)
		if !ok {
			return mcppkg.NewToolResultError("Invalid type for parameter format: expected string"), nil
		}
		format = formatStr
	}

	// Get options with authentication
	options := auth.CreateOptions(ctx)

	// First, pull the image
	img, err := crane.Pull(image, options...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error pulling image: %v", err)), nil
	}

	// Then save it in the requested format
	switch format {
	case "tarball":
		if err := crane.Save(img, image, output); err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error saving image: %v", err)), nil
		}
	case "legacy":
		if err := crane.SaveLegacy(img, image, output); err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error saving legacy image: %v", err)), nil
		}
	case "oci":
		if err := crane.SaveOCI(img, output); err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error saving OCI image: %v", err)), nil
		}
	default:
		return mcppkg.NewToolResultError(fmt.Sprintf("Unsupported format: %s", format)), nil
	}

	return mcppkg.NewToolResultText(fmt.Sprintf("Successfully pulled %s to %s", image, output)), nil
}
