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

// Package testutil provides testing utilities for crane MCP tools.
package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolTestCase defines a test case for an MCP tool.
type ToolTestCase struct {
	Name         string
	Arguments    map[string]interface{}
	ExpectError  bool
	ExpectedText string
}

// ToolHandlerFunc is the function signature for an MCP tool handler.
type ToolHandlerFunc func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// RunToolTests runs a series of test cases against a tool handler.
func RunToolTests(t *testing.T, handler ToolHandlerFunc, testCases []ToolTestCase) {
	t.Helper()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			req := mcp.CallToolRequest{}
			req.Params.Arguments = tc.Arguments

			result, err := handler(ctx, req)

			if tc.ExpectError {
				if err == nil && (result == nil || !result.IsError) {
					t.Errorf("Expected error but got success")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			// Check if there's an error in the result
			if result.IsError {
				t.Errorf("Unexpected error in result with content: %+v", result.Content)
				return
			}

			// If we expect specific text and have content
			if tc.ExpectedText != "" && len(result.Content) > 0 {
				textContent, ok := result.Content[0].(mcp.TextContent)
				if !ok {
					t.Errorf("Expected TextContent but got different type: %T", result.Content[0])
					return
				}

				if textContent.Text != tc.ExpectedText {
					t.Errorf("Expected text %q but got %q", tc.ExpectedText, textContent.Text)
				}
			}
		})
	}
}

// RunJSONToolTests runs tests for tools that return JSON results.
func RunJSONToolTests(t *testing.T, handler ToolHandlerFunc, testCases []ToolTestCase) {
	t.Helper()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx := context.Background()

			req := mcp.CallToolRequest{}
			req.Params.Arguments = tc.Arguments

			result, err := handler(ctx, req)

			if tc.ExpectError {
				if err == nil && (result == nil || !result.IsError) {
					t.Errorf("Expected error but got success")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			// Check if there's an error in the result
			if result.IsError {
				t.Errorf("Unexpected error in result with content: %+v", result.Content)
				return
			}

			// For JSON results, we expect valid JSON in the text content
			if len(result.Content) > 0 {
				textContent, ok := result.Content[0].(mcp.TextContent)
				if !ok {
					t.Errorf("Expected TextContent but got different type: %T", result.Content[0])
					return
				}

				// Try to parse the text as JSON
				var parsed interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &parsed); err != nil {
					t.Errorf("Result is not valid JSON: %s", textContent.Text)
				}

				// If we expect specific text
				if tc.ExpectedText != "" {
					if textContent.Text != tc.ExpectedText {
						t.Errorf("Expected JSON %q but got %q", tc.ExpectedText, textContent.Text)
					}
				}
			}
		})
	}
}

// CreateMockContext creates a mock context for testing.
func CreateMockContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

// AssertToolDefinition verifies that a tool definition has the expected properties.
func AssertToolDefinition(t *testing.T, tool mcp.Tool, expectedName string) {
	t.Helper()

	if tool.Name != expectedName {
		t.Errorf("Expected tool name %q but got %q", expectedName, tool.Name)
	}

	if tool.Description == "" {
		t.Errorf("Tool description should not be empty")
	}
}

// AssertToolResult verifies that a tool result contains the expected content.
func AssertToolResult(t *testing.T, result *mcp.CallToolResult, expectedContent string) {
	t.Helper()

	if result == nil {
		t.Fatalf("Result is nil")
	}

	if result.IsError {
		t.Errorf("Result contains an error with content: %+v", result.Content)
	}

	if len(result.Content) == 0 {
		t.Errorf("Result has no content")
		return
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("Expected TextContent but got different type: %T", result.Content[0])
		return
	}

	if diff := cmp.Diff(expectedContent, textContent.Text); diff != "" {
		t.Errorf("Result content mismatch (-want +got):\n%s", diff)
	}
}

// MockToolRequest creates a mock tool request with the given arguments.
func MockToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// MockJSONString creates a properly formatted JSON string for testing.
func MockJSONString(obj interface{}) string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal object to JSON: %v", err))
	}
	return string(bytes)
}
