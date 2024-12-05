#!/bin/bash
set -e
exec uvx mcp-server-sqlite --db-path test.db
