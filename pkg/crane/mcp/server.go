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

package mcp

import (
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/catalog"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/config"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/copy"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/digest"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/list"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/manifest"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/pull"
	"github.com/google/go-containerregistry/pkg/crane/mcp/tools/push"
	"github.com/mark3labs/mcp-go/server"
)

// Config contains configuration options for the MCP server.
type Config struct {
	// Name is the server name.
	Name string
	// Version is the server version.
	Version string
}

// DefaultConfig returns the default configuration for the server.
func DefaultConfig() Config {
	return Config{
		Name:    "crane-mcp",
		Version: "1.0.0",
	}
}

// Server is the crane MCP server.
type Server struct {
	mcpServer *server.MCPServer
	config    Config
}

// New creates a new crane MCP server.
func New(cfg Config) *Server {
	s := &Server{
		mcpServer: server.NewMCPServer(cfg.Name, cfg.Version),
		config:    cfg,
	}

	// Register all tools
	s.registerTools()

	return s
}

// registerTools registers all crane tools with the MCP server.
func (s *Server) registerTools() {
	// Register digest tool
	s.mcpServer.AddTool(digest.NewTool(), digest.Handle)

	// Register pull tool
	s.mcpServer.AddTool(pull.NewTool(), pull.Handle)

	// Register push tool
	s.mcpServer.AddTool(push.NewTool(), push.Handle)

	// Register copy tool
	s.mcpServer.AddTool(copy.NewTool(), copy.Handle)

	// Register catalog tool
	s.mcpServer.AddTool(catalog.NewTool(), catalog.Handle)

	// Register list tool
	s.mcpServer.AddTool(list.NewTool(), list.Handle)

	// Register config tool
	s.mcpServer.AddTool(config.NewTool(), config.Handle)

	// Register manifest tool
	s.mcpServer.AddTool(manifest.NewTool(), manifest.Handle)
}

// Serve starts the server with stdio.
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcpServer)
}

// Run starts the server and exits the program if an error occurs.
func (s *Server) Run() {
	if err := s.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
