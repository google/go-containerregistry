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

// Package config provides an MCP tool for getting the config of an image.
package config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/crane/mcp/auth"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
)

// NewTool creates a new config tool.
func NewTool() mcppkg.Tool {
	return mcppkg.NewTool("config",
		mcppkg.WithDescription("Get the config of an image"),
		mcppkg.WithString("image",
			mcppkg.Required(),
			mcppkg.Description("The image reference to get the config for"),
		),
		mcppkg.WithBoolean("pretty",
			mcppkg.Description("Pretty print the config JSON"),
			mcppkg.DefaultBool(true),
		),
	)
}

// Handle handles config tool requests.
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

	pretty, _ := request.Params.Arguments["pretty"].(bool)

	// Get options with authentication
	options := auth.CreateOptions(ctx)

	config, err := crane.Config(image, options...)
	if err != nil {
		return mcppkg.NewToolResultError(fmt.Sprintf("Error getting config: %v", err)), nil
	}

	var result []byte
	if pretty {
		var obj interface{}
		if err := json.Unmarshal(config, &obj); err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error parsing config: %v", err)), nil
		}
		result, err = json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return mcppkg.NewToolResultError(fmt.Sprintf("Error formatting config: %v", err)), nil
		}
	} else {
		result = config
	}

	// Return as JSON
	return mcppkg.NewToolResultText(string(result)), nil
}
