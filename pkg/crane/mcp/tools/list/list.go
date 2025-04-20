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

// Package list provides an MCP tool for listing tags in a repository.
package list

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/crane/mcp/auth"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// NewTool creates a new list tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("ls",
		mcppkg.WithDescription("List tags for a repository"),
		mcppkg.WithString("repository",
			mcppkg.Required(),
			mcppkg.Description("The repository to list tags from"),
		),
	)
}

// Handle handles list tool requests.
func Handle(ctx context.Context, request mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	// Check if required parameters are present
	repoVal, ok := request.Params.Arguments["repository"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: repository"), nil
	}
	repository, ok := repoVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter repository: expected string"), nil
	}

	// Get options with authentication
	options := auth.CreateOptions(ctx)
	o := crane.GetOptions(options...)

	repo, err := name.NewRepository(repository, o.Name...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error parsing repository: %v", err)), nil
	}

	// Use remote.List with auth options
	tags, err := remote.List(repo, o.Remote...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error listing tags: %v", err)), nil
	}

	result, err := json.Marshal(tags)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error marshaling result: %v", err)), nil
	}

	// Return as JSON
	return mcppkg.NewToolResultText(string(result)), nil
}
