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

// Package catalog provides an MCP tool for listing repositories in a registry.
package catalog

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

// NewTool creates a new catalog tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("catalog",
		mcppkg.WithDescription("List repositories in a registry"),
		mcppkg.WithString("registry",
			mcppkg.Required(),
			mcppkg.Description("The registry to list repositories from"),
		),
		mcppkg.WithBoolean("full-ref",
			mcppkg.Description("If true, print the full repository references"),
			mcppkg.DefaultBool(false),
		),
	)
}

// Handle handles catalog tool requests.
func Handle(ctx context.Context, request mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	// Check if required parameters are present
	registryVal, ok := request.Params.Arguments["registry"]
	if !ok {
		return mcppkg.NewToolResultError("Missing required parameter: registry"), nil
	}
	registry, ok := registryVal.(string)
	if !ok {
		return mcppkg.NewToolResultError("Invalid type for parameter registry: expected string"), nil
	}

	fullRef, _ := request.Params.Arguments["full-ref"].(bool)

	// Get options with authentication
	options := auth.CreateOptions(ctx)
	o := crane.GetOptions(options...)

	reg, err := name.NewRegistry(registry, o.Name...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error parsing registry: %v", err)), nil
	}

	puller, err := remote.NewPuller(o.Remote...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error creating puller: %v", err)), nil
	}

	catalogger, err := puller.Catalogger(ctx, reg)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error reading from registry: %v", err)), nil
	}

	var repos []string
	for catalogger.HasNext() {
		reposList, err := catalogger.Next(ctx)
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error reading repositories: %v", err)), nil
		}

		for _, repo := range reposList.Repos {
			if fullRef {
				repos = append(repos, fmt.Sprintf("%s/%s", registry, repo))
			} else {
				repos = append(repos, repo)
			}
		}
	}

	result, err := json.Marshal(repos)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error marshaling result: %v", err)), nil
	}

	// Return as JSON
	return mcppkg.NewToolResultText(string(result)), nil
}
