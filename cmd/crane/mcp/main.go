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

// Binary crane-mcp provides an MCP server for crane commands.
//
// This binary implements the Model Context Protocol, allowing AI assistants
// to interact with OCI registries using crane functionality.
package main

import (
	"log"

	"github.com/google/go-containerregistry/pkg/crane/mcp"
)

func main() {
	// Create a new server with default configuration
	svc := mcp.New(mcp.DefaultConfig())

	// Start the server
	log.Println("Starting crane MCP server...")
	svc.Run()
}
