# gomcp - Go Model Context Protocol Bridge For Ollama 🦙🚀

A high-performance Go implementation of a bridge connecting Model Context Protocol (MCP) servers to LLMs like Ollama. This bridge enables seamless integration between language models and external tools and data sources.

**This is very much ALPHA quality software, perhaps not even that. It's something I hacked up in a day to get a proof of concept working.**

_Note: The flagrant abuse of emojis is thanks to the AI that wrote this readme 😂_

## Features

- 🔄 Full MCP protocol support via mcp-go
- 🛠️ Multiple built-in tools (Database, HTTP, Filesystem, Time)
- 💾 SQLite database integration
- 🔍 Goroutine leak detection
- 🌐 Interactive CLI and HTTP server modes
- ⚡ Efficient error handling with proper types
- 🔒 Safe configuration management
- 🧩 Extensible tool system

## Installation

```bash
# Using go install
go install github.com/sammcj/gomcp/cmd/gomcp@latest

# Or clone and build
git clone https://github.com/sammcj/gomcp.git
cd gomcp
make build
```

## Quick Start

1. Initialise configuration:

```bash
gomcp -init
```
This creates a default configuration file at `~/.config/gomcp/config.yaml`.

1. Edit the configuration file to match your setup:

```yaml
llm:
  model: "qwen2.5-coder:7b-instruct-q6_K"  # Your Ollama model
  endpoint: "http://localhost:11434"
  api_key: ""  # Optional
  system_prompt: |
    You are a helpful assistant with access to various tools.

    [Tools]
    When using the database tool:
    1. Use exact column names from the schema
    2. Write valid SQL queries
    3. Remember this is SQLite

mcp:
  command: "uvx"
  arguments:
    - "mcp-server-sqlite"
    - "--db-path"
    - "test.db"

database:
  path: "test.db"

logging:
  level: "info"
  format: "text"

server:
  enable: false
  host: "localhost"
  port: 8080
```

Run in interactive mode:

```bash
gomcp
```

Run as a server:

```bash
gomcp -server
```

## Usage Examples

### Interactive Mode

```bash
$ gomcp
MCP LLM Bridge
Type 'quit' or press Ctrl+C to exit

Enter your message: What tables are available in the database?
Response: Let me query the database schema for you...
[Database schema information follows]

Enter your message: Show me the top 5 most expensive products
Response: I'll query the products table for that information...
[Query results follow]
```

### Server Mode

Start the server:
```bash
gomcp -server
```

Make requests:
```bash
# Send a chat message
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "What are the most expensive products?"}'

# Check server health
curl http://localhost:8080/health
```

### Available Tools

#### Database Tool

- Executes SQL queries against SQLite databases
- Read-only operations (SELECT queries only)
- Automatic schema detection

#### HTTP Tool

- Makes HTTP requests to allowed domains
- Supports GET, POST, PUT, DELETE methods
- Domain allowlist for security

#### Filesystem Tool

- List directory contents
- Read file contents
- Check file existence
- Get file information
- Path restrictions for security

#### Time Tool

- Get current time
- Parse timestamps
- Format times
- Compare timestamps

## Development

### Prerequisites

- Go 1.23 or later
- SQLite3
- Make

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run linting
make lint

# Clean build artifacts
make clean
```

### Adding New Tools

Tools can be added by implementing the tool interface:

```go
type Tool interface {
    GetToolSpec() mcp.Tool
    Execute(params map[string]interface{}) (interface{}, error)
}
```

### Error Handling

The project uses custom error types in the `types` package:

- `ConfigError` for configuration issues
- `BridgeError` for bridge operations
- `LLMError` for LLM-related errors
- `ToolError` for tool execution issues
- `DatabaseError` for database operations

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Create a Pull Request

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE.md) file for details.

## Acknowledgments

- [bartolli/mcp-llm-bridge](https://github.com/bartolli/mcp-llm-bridge) for which I took inspiration
- [Model Context Protocol](https://modelcontextprotocol.io/) specification
- [mcp-go](https://github.com/mark3labs/mcp-go) package
- [Ollama](https://ollama.ai/) project
