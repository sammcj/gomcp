#!/bin/bash
set -e

# Get the config file path
CONFIG_DIR="$HOME/.config/gomcp"
CONFIG_FILE="$CONFIG_DIR/config.yaml"

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Config file not found at $CONFIG_FILE"
    exit 1
fi

# Function to start a server in the background
start_server() {
    local name="$1"
    local cmd="$2"
    shift 2
    local args=("$@")

    echo "Starting MCP server: $name"
    echo "Command: $cmd ${args[*]}"

    # Start the server in the background
    if [ "$name" = "bybit" ]; then
        # For bybit-mcp, we need to run it in its directory
        (cd /Users/samm/git/sammcj/bybit-mcp && pnpm run serve) &
    else
        # For other servers, run normally
        $cmd "${args[@]}" &
    fi

    # Store the PID
    echo $! > "/tmp/gomcp-$name.pid"
}

# Start all configured servers
echo "Starting MCP servers..."

# Default SQLite server
start_server "sqlite" "uvx" "mcp-server-sqlite" "--db-path" "test.db"

# Start bybit-mcp server
start_server "bybit" "pnpm" "run" "serve"

# Wait for all background processes
wait
