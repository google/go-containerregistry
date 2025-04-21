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

// Package push provides an MCP tool for pushing container images.
package push

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/crane/mcp/auth"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// NewTool creates a new push tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("push",
		mcppkg.WithDescription("Push a container image from a tarball to a registry"),
		mcppkg.WithString("tarball",
			mcppkg.Required(),
			mcppkg.Description("Path to the image tarball to push"),
		),
		mcppkg.WithString("image",
			mcppkg.Required(),
			mcppkg.Description("The image reference to push to"),
		),
		mcppkg.WithBoolean("index",
			mcppkg.Description("Push a collection of images as a single index"),
			mcppkg.DefaultBool(false),
		),
	)
}

// Handle handles push tool requests.
func Handle(ctx context.Context, request mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	// Check if required parameters are present
	tarballVal, ok := request.Params.Arguments["tarball"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: tarball"), nil
	}
	tarballPath, ok := tarballVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter tarball: expected string"), nil
	}

	imageVal, ok := request.Params.Arguments["image"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: image"), nil
	}
	image, ok := imageVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter image: expected string"), nil
	}

	isIndex, _ := request.Params.Arguments["index"].(bool)

	// Get options with authentication
	options := auth.CreateOptions(ctx)

	// Determine if the path is a directory (OCI layout) or a file (tarball)
	stat, err := os.Stat(tarballPath)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error accessing path: %v", err)), nil
	}

	var digest string
	if !stat.IsDir() {
		// Handle tarball
		img, err := crane.Load(tarballPath)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error loading image: %v", err)), nil
		}

		err = crane.Push(img, image, options...)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error pushing image: %v", err)), nil
		}

		// Get the digest after pushing
		h, err := img.Digest()
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error getting digest: %v", err)), nil
		}

		ref, err := name.ParseReference(image)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error parsing reference: %v", err)), nil
		}
		digest = ref.Context().Digest(h.String()).String()
	} else if isIndex {
		// Handle OCI layout as index
		p, err := layout.FromPath(tarballPath)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error loading OCI layout: %v", err)), nil
		}

		idx, err := p.ImageIndex()
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error loading image index: %v", err)), nil
		}

		ref, err := name.ParseReference(image)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error parsing reference: %v", err)), nil
		}

		// Get remote options with authentication from our crane options
		o := crane.GetOptions(options...)
		if err := remote.WriteIndex(ref, idx, o.Remote...); err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error pushing image index: %v", err)), nil
		}

		h, err := idx.Digest()
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error getting digest: %v", err)), nil
		}
		digest = ref.Context().Digest(h.String()).String()
	} else {
		return mcppkg.NewToolResultError("Directory provided but --index not specified. Use --index with OCI layouts."), nil
	}

	return mcppkg.NewToolResultText(digest), nil
}
