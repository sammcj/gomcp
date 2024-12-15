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
- 🔌 Multi-server support with dynamic tool discovery
- 📈 Bybit cryptocurrency exchange integration

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

2. Edit the configuration file to match your setup. Here's an example with multiple MCP servers:

```yaml
llm:
  model: "qwen2.5-coder-7b-instruct-128k:q6_k"  # Your Ollama model
  endpoint: "http://localhost:11434/api"
  api_key: ""  # Optional
  system_prompt: |
    You are a helpful assistant with access to various tools.

    [Tools]
    When using the database tool:
    1. Use exact column names from the schema
    2. Write valid SQL queries
    3. Remember this is SQLite

    When using bybit tools:
    1. All amounts are in USD
    2. Follow proper position sizing and risk management
    3. Always verify order details before execution

mcp_servers:
  - name: "sqlite"
    command: "uvx"
    arguments:
      - "mcp-server-sqlite"
      - "--db-path"
      - "test.db"

  - name: "bybit"
    command: "/bin/sh"
    arguments:
      - "-c"
      - "cd /path/to/bybit-mcp && pnpm run serve"  # Replace with your bybit-mcp path
    env:
      BYBIT_API_KEY: ""      # Add your Bybit API **READ ONLY** key here
      BYBIT_API_SECRET: ""   # Add your Bybit API **READ ONLY** secret here
      BYBIT_USE_TESTNET: "true"  # Set to false for production

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

3. Run in interactive mode:

```bash
gomcp
```

4. Run as a server:

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

Enter your message: Show me the current BTC/USD price on Bybit
Response: I'll fetch the current price from Bybit...
[Price information follows]
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

#### Bybit Tools

The following tools are available when the bybit-mcp server is configured:

- `get_ticker`: Get real-time ticker information for a trading pair (e.g., "BTCUSDT")
  ```json
  {
    "name": "get_ticker",
    "arguments": {
      "symbol": "BTCUSDT",
      "category": "spot"
    }
  }
  ```

- `get_orderbook`: Get orderbook (market depth) data for a trading pair
  ```json
  {
    "name": "get_orderbook",
    "arguments": {
      "symbol": "BTCUSDT",
      "category": "spot",
      "limit": 25
    }
  }
  ```

- `get_kline`: Get kline/candlestick data for a trading pair
  ```json
  {
    "name": "get_kline",
    "arguments": {
      "symbol": "BTCUSDT",
      "category": "spot",
      "interval": "1"
    }
  }
  ```

- `get_market_info`: Get detailed market information for trading pairs
- `get_trades`: Get recent trades for a trading pair
- `get_instrument_info`: Get detailed instrument information for a specific trading pair
- `get_wallet_balance`: Get wallet balance information for the authenticated user
- `get_positions`: Get current positions information for the authenticated user
- `get_order_history`: Get order history for the authenticated user

Common Parameters:
- `symbol`: Trading pair in the format "BTCUSDT", "ETHUSDT", etc.
- `category`: Market category, usually "spot" for spot trading
- `limit`: Number of records to return (varies by endpoint)

For detailed usage of each Bybit tool, refer to the [bybit-mcp documentation](https://github.com/sammcj/bybit-mcp).

## Development

### Prerequisites

- Go 1.23 or later
- SQLite3
- Make
- Node.js and pnpm (for bybit-mcp)

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

### Adding New MCP Servers

The bridge supports multiple MCP servers running simultaneously. To add a new server:

1. Install the MCP server
2. Add the server configuration to `~/.config/gomcp/config.yaml`:
```yaml
mcp_servers:
  - name: "your-server"
    command: "/bin/sh"  # Use shell for complex commands
    arguments:
      - "-c"
      - "cd /path/to/server && command-to-run"
    env:
      KEY1: "value1"
      KEY2: "value2"
```
3. The bridge will automatically discover and expose the server's tools

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

- [LICENSE](LICENSE.md)

## Acknowledgments

- [bartolli/mcp-llm-bridge](https://github.com/bartolli/mcp-llm-bridge) for which I took inspiration
- [Model Context Protocol](https://modelcontextprotocol.io/) specification
- [mcp-go](https://github.com/mark3labs/mcp-go) package
- [Ollama](https://ollama.ai/) project
