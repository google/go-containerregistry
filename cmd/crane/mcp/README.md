# Crane MCP Server

This is an implementation of the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) for the `crane` CLI tool. It allows AI assistants and other MCP clients to interact with container registries using the functionality provided by the `crane` command line tool.

## Overview

The server exposes several tools from the `crane` CLI as MCP tools, including:

- `digest`: Get the digest of a container image
- `pull`: Pull a container image and save it as a tarball
- `push`: Push a container image from a tarball to a registry
- `copy`: Copy an image from one registry to another
- `catalog`: List repositories in a registry
- `ls`: List tags for a repository
- `config`: Get the config of an image
- `manifest`: Get the manifest of an image

## Usage

### Building

```bash
go build -o crane-mcp
```

### Running

```bash
./crane-mcp
```

The server communicates through stdin/stdout according to the MCP protocol. It can be integrated with any MCP client.

## Authentication

The server uses the same authentication mechanisms as the `crane` CLI. For private registries, you may need to:

1. Log in to the registry with `docker login` or `crane auth login`
2. Set up appropriate environment variables or credentials files

## Example Client Requests

To get the digest of an image:

```json
{
  "type": "call_tool",
  "id": "1",
  "params": {
    "name": "digest",
    "arguments": {
      "image": "docker.io/library/ubuntu:latest",
      "full-ref": true
    }
  }
}
```

To copy an image between registries:

```json
{
  "type": "call_tool",
  "id": "2",
  "params": {
    "name": "copy",
    "arguments": {
      "source": "docker.io/library/nginx:latest",
      "destination": "otherregistry.io/nginx:latest"
    }
  }
}
```

## License

Licensed under the Apache License, Version 2.0. See LICENSE file for details.
