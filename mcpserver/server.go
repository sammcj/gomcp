package mcpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	_ "github.com/mattn/go-sqlite3"
)

type MCPServer struct {
	server *server.MCPServer
	db     *sql.DB
	logger *log.Logger
}

func NewMCPServer(dbPath string, logger *log.Logger) *MCPServer {
	// Open SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Printf("Failed to open database: %v", err)
		return nil
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Printf("Failed to ping database: %v", err)
		db.Close()
		return nil
	}

	logger.Printf("Successfully connected to database: %s", dbPath)

	s := &MCPServer{
		server: server.NewMCPServer(
			"gomcp-sqlite-server",
			"1.0.0",
			server.WithToolCapabilities(true),
			server.WithLogging(),
		),
		db:     db,
		logger: logger,
	}

	// Add query tool
	s.server.AddTool(mcp.Tool{
		Name:        "query_database",
		Description: "Execute a SQL query against the SQLite database",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "SQL query to execute",
				},
			},
			Required: []string{"query"},
		},
	}, s.handleQueryTool)

	// Add notification handler
	s.server.AddNotificationHandler(s.handleNotification)

	logger.Printf("MCP server created with tool: query_database")
	return s
}

func (s *MCPServer) handleQueryTool(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	query, ok := arguments["query"].(string)
	if !ok {
		s.logger.Printf("Invalid query argument: %v", arguments)
		return nil, fmt.Errorf("invalid query argument")
	}

	s.logger.Printf("Executing query: %s", query)

	// Execute query
	rows, err := s.db.Query(query)
	if err != nil {
		s.logger.Printf("Failed to execute query: %v", err)
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		s.logger.Printf("Failed to get column names: %v", err)
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}

	// Prepare result
	var results []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Scan the result into the values
		if err := rows.Scan(valuePtrs...); err != nil {
			s.logger.Printf("Failed to scan row: %v", err)
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			row[col] = v
		}
		results = append(results, row)
	}

	// Convert results to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		s.logger.Printf("Failed to marshal results: %v", err)
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	s.logger.Printf("Query executed successfully, returned %d rows", len(results))
	return &mcp.CallToolResult{
		Content: []interface{}{
			mcp.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

func (s *MCPServer) handleNotification(notification mcp.JSONRPCNotification) {
	s.logger.Printf("Received notification: %s", notification.Method)
}

func (s *MCPServer) Serve() error {
	s.logger.Println("Starting MCP server...")
	if err := server.ServeStdio(s.server); err != nil {
		s.logger.Printf("Server error: %v", err)
		return fmt.Errorf("server error: %w", err)
	}
	s.logger.Println("MCP server stopped")
	return nil
}

func (s *MCPServer) Close() error {
	s.logger.Println("Closing MCP server...")
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Printf("Failed to close database: %v", err)
			return fmt.Errorf("failed to close database: %w", err)
		}
	}
	s.logger.Println("MCP server closed")
	return nil
}
