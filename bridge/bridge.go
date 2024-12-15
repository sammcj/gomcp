package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/gomcp/config"
	"github.com/sammcj/gomcp/llm"
	"github.com/sammcj/gomcp/tools"
	"github.com/sammcj/gomcp/types"
)

// Bridge manages communication between MCP and LLM
type Bridge struct {
	ctx       context.Context
	cancel    context.CancelFunc
	llmClient *llm.Client
	tools     []mcp.Tool
	toolMap   map[string]string     // Maps sanitized tool names to original names
	serverMap map[string]*MCPClient // Maps server names to their clients
	dbTool    *tools.DatabaseTool
	logger    *log.Logger
	config    *config.Config
	debug     bool
	mu        sync.RWMutex
}

// New creates a new Bridge instance
func New(cfg *config.Config, logger *log.Logger) (*Bridge, error) {
	ctx, cancel := context.WithCancel(context.Background())

	debug := strings.ToLower(cfg.Logging.Level) == "debug"

	if debug {
		logger.Println("Creating new bridge...")
	}

	// Create LLM client
	if debug {
		logger.Printf("Creating LLM client with endpoint: %s", cfg.LLM.Endpoint)
	}
	llmClient := llm.New(cfg.LLM.Endpoint, cfg.LLM.Model, cfg.LLM.SystemPrompt)
	if debug {
		logger.Println("LLM client created")
	}

	// Create database tool
	if debug {
		logger.Printf("Creating database tool with path: %s", cfg.Database.Path)
	}
	dbTool, err := tools.NewDatabaseTool(cfg.Database.Path)
	if err != nil {
		cancel()
		logger.Printf("Failed to create database tool: %v", err)
		return nil, &types.BridgeError{
			Operation: "create_db_tool",
			Message:   "failed to create database tool",
			Err:       err,
		}
	}
	if debug {
		logger.Println("Database tool created successfully")
	}

	// Create tool definition for database
	queryTool := mcp.Tool{
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
	}

	bridge := &Bridge{
		ctx:       ctx,
		cancel:    cancel,
		llmClient: llmClient,
		tools:     []mcp.Tool{queryTool},
		toolMap:   make(map[string]string),
		serverMap: make(map[string]*MCPClient),
		dbTool:    dbTool,
		logger:    logger,
		config:    cfg,
		debug:     debug,
	}

	if debug {
		logger.Println("Bridge instance created successfully")
	}
	return bridge, nil
}

// Initialize sets up the bridge and connects to all configured MCP servers
func (b *Bridge) Initialize() error {
	if b.debug {
		b.logger.Println("Starting bridge initialization...")
	}

	// Register built-in tools
	if b.debug {
		b.logger.Println("Registering built-in tools...")
	}
	b.toolMap["query_database"] = "query_database"

	// Initialize MCP servers
	if b.debug {
		b.logger.Printf("Initializing %d MCP servers...", len(b.config.MCPServers))
	}

	for _, serverCfg := range b.config.MCPServers {
		if b.debug {
			b.logger.Printf("Initializing MCP server: %s", serverCfg.Name)
		}

		// Create client with environment variables
		client, err := NewMCPClient(serverCfg.Command, serverCfg.Arguments, b.logger)
		if err != nil {
			return fmt.Errorf("failed to create MCP client for %s: %w", serverCfg.Name, err)
		}

		// Initialize the client
		if b.debug {
			b.logger.Printf("Initializing MCP client for %s...", serverCfg.Name)
		}
		_, err = client.Initialize(b.ctx, mcp.InitializeRequest{})
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to initialize MCP client for %s: %w", serverCfg.Name, err)
		}

		// List tools from the server
		if b.debug {
			b.logger.Printf("Listing tools for %s...", serverCfg.Name)
		}
		toolsResult, err := client.ListTools(b.ctx, mcp.ListToolsRequest{})
		if err != nil {
			client.Close()
			return fmt.Errorf("failed to list tools for %s: %w", serverCfg.Name, err)
		}

		// Register server's tools
		for _, tool := range toolsResult.Tools {
			sanitizedName := sanitizeToolName(tool.Name)
			b.toolMap[sanitizedName] = fmt.Sprintf("%s/%s", serverCfg.Name, tool.Name)
			b.tools = append(b.tools, tool)
			if b.debug {
				b.logger.Printf("Registered tool %s from server %s", tool.Name, serverCfg.Name)
			}
		}

		// Store the client
		b.mu.Lock()
		b.serverMap[serverCfg.Name] = client
		b.mu.Unlock()

		if b.debug {
			b.logger.Printf("MCP server %s initialized with %d tools", serverCfg.Name, len(toolsResult.Tools))
		}
	}

	// Set tools in LLM client
	if b.debug {
		b.logger.Printf("Setting %d tools in LLM client...", len(b.tools))
	}
	if err := b.llmClient.SetTools(b.tools); err != nil {
		return fmt.Errorf("failed to set tools in LLM client: %w", err)
	}

	if b.debug {
		b.logger.Println("Bridge initialization completed successfully")
	}
	return nil
}

// ProcessMessage handles a message from the user through the LLM and tools
func (b *Bridge) ProcessMessage(msg string) (string, error) {
	if b.debug {
		b.logger.Printf("Processing message: %s", msg)
	}
	_, cancel := context.WithTimeout(b.ctx, 300*time.Second)
	defer cancel()

	var response *types.LLMResponse
	var err error

	// Implement retry with backoff
	backoff := time.Second
	for attempts := 0; attempts < 3; attempts++ {
		if b.debug {
			b.logger.Printf("Generating LLM response (attempt %d/3)", attempts+1)
		}
		response, err = b.generateLLMResponse(msg)
		if err == nil {
			break
		}

		if !isRetryableError(err) {
			if b.debug {
				b.logger.Printf("Non-retryable error encountered: %v", err)
			}
			return "", err
		}

		if b.debug {
			b.logger.Printf("Retrying after error: %v (attempt %d/3)", err, attempts+1)
		}
		time.Sleep(backoff)
		backoff *= 2
	}

	if err != nil {
		return "", &types.BridgeError{
			Operation: "process_message",
			Message:   "failed after retry attempts",
			Err:       err,
		}
	}

	// Process tool calls if present
	if len(response.ToolCalls) > 0 {
		toolResults, err := b.handleToolCalls(response.ToolCalls)
		if err != nil {
			if b.debug {
				b.logger.Printf("Tool execution failed: %v", err)
			}
			return "", &types.BridgeError{
				Operation: "handle_tools",
				Message:   "tool execution failed",
				Err:       err,
			}
		}

		if len(toolResults) > 0 {
			return toolResults[0]["output"].(string), nil
		}
	}

	// Clean up any unwanted tags in the response
	content := strings.ReplaceAll(response.Content, "<|im_start|>", "")
	content = strings.ReplaceAll(content, "<|im_end|>", "")
	content = strings.TrimSpace(content)

	return content, nil
}

// handleToolCalls processes tool invocations from the LLM
func (b *Bridge) handleToolCalls(toolCalls []types.ToolCall) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, call := range toolCalls {
		// Get original tool name and server
		mcpName, ok := b.toolMap[call.Function.Name]
		if !ok {
			if b.debug {
				b.logger.Printf("Unknown tool requested: %s", call.Function.Name)
			}
			return nil, fmt.Errorf("unknown tool: %s", call.Function.Name)
		}

		// Handle built-in database tool
		if mcpName == "query_database" {
			result, err := b.handleDatabaseTool(call)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
			continue
		}

		// Parse server and tool name
		parts := strings.SplitN(mcpName, "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid tool mapping: %s", mcpName)
		}
		serverName, toolName := parts[0], parts[1]

		// Get the MCP client
		b.mu.RLock()
		client, ok := b.serverMap[serverName]
		b.mu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("unknown MCP server: %s", serverName)
		}

		// Convert numeric arguments to strings for enum fields
		convertedArgs := make(map[string]interface{})
		for k, v := range call.Function.Arguments {
			switch val := v.(type) {
			case float64:
				// Convert numeric values to strings for known numeric enum fields
				if k == "limit" || k == "interval" {
					convertedArgs[k] = fmt.Sprintf("%v", val)
				} else {
					convertedArgs[k] = val
				}
			default:
				convertedArgs[k] = val
			}
		}

		// Execute the tool with converted arguments
		if b.debug {
			b.logger.Printf("Executing tool %s on server %s with arguments: %v", toolName, serverName, convertedArgs)
		}
		result, err := client.CallTool(b.ctx, mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
			}{
				Name:      toolName,
				Arguments: convertedArgs,
			},
		})
		if err != nil {
			if b.debug {
				b.logger.Printf("Tool execution failed: %v", err)
			}
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		// Format the result
		formattedResult := b.formatToolResult(result)
		results = append(results, map[string]interface{}{
			"tool_call_id": call.ID,
			"output":       formattedResult,
		})
	}

	return results, nil
}

// handleDatabaseTool processes database tool calls
func (b *Bridge) handleDatabaseTool(call types.ToolCall) (map[string]interface{}, error) {
	query, ok := call.Function.Arguments["query"].(string)
	if !ok {
		if b.debug {
			b.logger.Printf("Invalid query argument: %v", call.Function.Arguments)
		}
		return nil, fmt.Errorf("invalid query argument")
	}

	if b.debug {
		b.logger.Printf("Executing database query: %s", query)
	}
	result, err := b.dbTool.Execute(map[string]interface{}{"query": query})
	if err != nil {
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	formattedResult := b.formatDatabaseResult(result)
	return map[string]interface{}{
		"tool_call_id": call.ID,
		"output":       formattedResult,
	}, nil
}

// formatToolResult formats the result from an MCP tool call
func (b *Bridge) formatToolResult(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	var output strings.Builder
	for _, content := range result.Content {
		switch v := content.(type) {
		case map[string]interface{}:
			if text, ok := v["text"].(string); ok {
				output.WriteString(text)
				output.WriteString("\n")
			} else if data, err := json.MarshalIndent(v, "", "  "); err == nil {
				output.Write(data)
				output.WriteString("\n")
			}
		default:
			if data, err := json.MarshalIndent(v, "", "  "); err == nil {
				output.Write(data)
				output.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(output.String())
}

// formatDatabaseResult formats database query results
func (b *Bridge) formatDatabaseResult(result interface{}) string {
	switch v := result.(type) {
	case []map[string]interface{}:
		if len(v) == 0 {
			return ""
		}

		// Get all unique column names from all rows
		columnSet := make(map[string]struct{})
		for _, row := range v {
			for col := range row {
				columnSet[col] = struct{}{}
			}
		}

		// Convert to sorted slice
		var columns []string
		for col := range columnSet {
			columns = append(columns, col)
		}
		sort.Strings(columns)

		// Calculate column widths
		colWidths := make(map[string]int)
		for _, col := range columns {
			colWidths[col] = len(col)
			for _, row := range v {
				if val, ok := row[col]; ok {
					width := len(fmt.Sprintf("%v", val))
					if width > colWidths[col] {
						colWidths[col] = width
					}
				}
			}
		}

		// Build formatted output
		var sb strings.Builder

		// Header
		for _, col := range columns {
			sb.WriteString(fmt.Sprintf("%-*s  ", colWidths[col], strings.ToUpper(col)))
		}
		sb.WriteString("\n")

		// Separator
		for _, col := range columns {
			sb.WriteString(strings.Repeat("-", colWidths[col]))
			sb.WriteString("  ")
		}
		sb.WriteString("\n")

		// Data rows
		for _, row := range v {
			for _, col := range columns {
				if val, ok := row[col]; ok {
					sb.WriteString(fmt.Sprintf("%-*v  ", colWidths[col], val))
				} else {
					sb.WriteString(fmt.Sprintf("%-*s  ", colWidths[col], ""))
				}
			}
			sb.WriteString("\n")
		}

		return sb.String()
	default:
		// For other types, use standard JSON marshaling
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			if b.debug {
				b.logger.Printf("Failed to marshal result: %v", err)
			}
			return fmt.Sprintf("Error formatting result: %v", err)
		}
		return string(jsonData)
	}
}

// Close cleans up resources
func (b *Bridge) Close() error {
	b.cancel()

	var errs []error

	// Close database tool
	if err := b.dbTool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("database tool: %w", err))
	}

	// Close all MCP clients
	b.mu.Lock()
	for name, client := range b.serverMap {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("MCP server %s: %w", name, err))
		}
	}
	b.serverMap = nil
	b.mu.Unlock()

	// Clear other resources
	b.toolMap = nil
	b.tools = nil

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}

	return nil
}

// Helper functions

// sanitizeToolName converts a tool name to a format compatible with LLMs
func sanitizeToolName(name string) string {
	// Replace characters that might cause issues
	sanitized := ""
	for _, r := range name {
		if r == '-' || r == ' ' {
			sanitized += "_"
		} else {
			sanitized += string(r)
		}
	}
	return sanitized
}

// generateLLMResponse sends a message to the LLM and gets a response
func (b *Bridge) generateLLMResponse(msg string) (*types.LLMResponse, error) {
	resp, err := b.llmClient.GenerateResponse(msg)
	if err != nil {
		if b.debug {
			b.logger.Printf("LLM response generation failed: %v", err)
		}
		return nil, err
	}
	return resp, nil
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network/timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// Add other retryable error types as needed
	return false
}
