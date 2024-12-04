package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
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
	toolMap   map[string]string
	dbTool    *tools.DatabaseTool
	logger    *log.Logger
	config    *config.Config
}

// New creates a new Bridge instance
func New(cfg *config.Config, logger *log.Logger) (*Bridge, error) {
	ctx, cancel := context.WithCancel(context.Background())

	logger.Println("Creating new bridge...")

	// Create LLM client
	logger.Printf("Creating LLM client with endpoint: %s", cfg.LLM.Endpoint)
	llmClient := llm.New(cfg.LLM.Endpoint, cfg.LLM.Model, cfg.LLM.SystemPrompt)
	logger.Println("LLM client created")

	// Create database tool
	logger.Printf("Creating database tool with path: %s", cfg.Database.Path)
	dbTool, err := tools.NewDatabaseTool(cfg.Database.Path)
	if err != nil {
		cancel()
		logger.Printf("Failed to create database tool: %v", err)
		return nil, &types.BridgeError{
			Operation: "create_db_tool",
			Message:   "failed to create database tool",
			Err:      err,
		}
	}
	logger.Println("Database tool created successfully")

	// Create tool definition
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
		dbTool:    dbTool,
		logger:    logger,
		config:    cfg,
	}

	logger.Println("Bridge instance created successfully")
	return bridge, nil
}

// Initialize sets up the bridge
func (b *Bridge) Initialize() error {
	b.logger.Println("Starting bridge initialization...")

	// Register tools
	b.logger.Println("Registering tools...")
	b.toolMap["query_database"] = "query_database"

	// Set tools in LLM client
	b.logger.Println("Setting tools in LLM client...")
	if err := b.llmClient.SetTools(b.tools); err != nil {
		return fmt.Errorf("failed to set tools in LLM client: %w", err)
	}

	b.logger.Println("Bridge initialization completed successfully")
	return nil
}

// ProcessMessage handles a message from the user through the LLM and tools
func (b *Bridge) ProcessMessage(msg string) (string, error) {
	b.logger.Printf("Processing message: %s", msg)
	_, cancel := context.WithTimeout(b.ctx, 300*time.Second)
	defer cancel()

	var response *types.LLMResponse
	var err error
	var lastToolResult string

	// Implement retry with backoff
	backoff := time.Second
	for attempts := 0; attempts < 3; attempts++ {
		b.logger.Printf("Generating LLM response (attempt %d/3)", attempts+1)
		response, err = b.generateLLMResponse(msg)
		if err == nil {
			break
		}

		if !isRetryableError(err) {
			b.logger.Printf("Non-retryable error encountered: %v", err)
			return "", err
		}

		b.logger.Printf("Retrying after error: %v (attempt %d/3)", err, attempts+1)
		time.Sleep(backoff)
		backoff *= 2
	}

	if err != nil {
		return "", &types.BridgeError{
			Operation: "process_message",
			Message:   "failed after retry attempts",
			Err:      err,
		}
	}

	b.logger.Printf("LLM response received: %+v", response)

	// Process tool calls if present
	for len(response.ToolCalls) > 0 {
		b.logger.Printf("Processing %d tool calls", len(response.ToolCalls))
		toolResults, err := b.handleToolCalls(response.ToolCalls)
		if err != nil {
			b.logger.Printf("Tool execution failed: %v", err)
			return "", &types.BridgeError{
				Operation: "handle_tools",
				Message:   "tool execution failed",
				Err:      err,
			}
		}

		if len(toolResults) > 0 {
			lastToolResult = toolResults[0]["output"].(string)
		}

		b.logger.Printf("Tool execution successful, continuing with results")
		response, err = b.generateLLMResponseWithTools(toolResults)
		if err != nil {
			b.logger.Printf("LLM continuation failed: %v", err)
			return "", &types.BridgeError{
				Operation: "continue_with_tools",
				Message:   "LLM continuation failed",
				Err:      err,
			}
		}

		// If the LLM response is empty or just contains tags, use the formatted tool result
		if response.Content == "" || response.Content == "<|im_start|>" || response.Content == "<|im_end|>" {
			response.Content = lastToolResult
			break
		}
	}

	// Clean up any unwanted tags in the response
	content := strings.ReplaceAll(response.Content, "<|im_start|>", "")
	content = strings.ReplaceAll(content, "<|im_end|>", "")
	content = strings.TrimSpace(content)

	// If content is still empty, use the last tool result
	if content == "" && lastToolResult != "" {
		content = lastToolResult
	}

	b.logger.Printf("Final response: %s", content)
	return content, nil
}

// handleToolCalls processes tool invocations from the LLM
func (b *Bridge) handleToolCalls(toolCalls []types.ToolCall) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, call := range toolCalls {
		b.logger.Printf("Processing tool call: %s", call.Function.Name)

		// Get original tool name
		mcpName, ok := b.toolMap[call.Function.Name]
		if !ok {
			b.logger.Printf("Unknown tool requested: %s", call.Function.Name)
			return nil, fmt.Errorf("unknown tool: %s", call.Function.Name)
		}

		// Execute tool directly
		var result interface{}
		var err error

		if mcpName == "query_database" {
			query, ok := call.Function.Arguments["query"].(string)
			if !ok {
				b.logger.Printf("Invalid query argument: %v", call.Function.Arguments)
				return nil, fmt.Errorf("invalid query argument")
			}

			b.logger.Printf("Executing database query: %s", query)
			result, err = b.dbTool.Execute(map[string]interface{}{"query": query})
		} else {
			b.logger.Printf("Unknown tool: %s", mcpName)
			return nil, fmt.Errorf("unknown tool: %s", mcpName)
		}

		if err != nil {
			b.logger.Printf("Tool execution failed: %v", err)
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		// Format the result for better readability
		var formattedResult string
		switch v := result.(type) {
		case []map[string]interface{}:
			if len(v) > 0 {
				// Get column names from the first row and sort them logically
				columns := []string{"id", "user_id", "product", "price", "created_at"}

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

				// Build header
				var sb strings.Builder
				for _, col := range columns {
					sb.WriteString(fmt.Sprintf("%-*s  ", colWidths[col], strings.ToUpper(col)))
				}
				sb.WriteString("\n")

				// Add separator line
				for _, col := range columns {
					sb.WriteString(strings.Repeat("-", colWidths[col]))
					sb.WriteString("  ")
				}
				sb.WriteString("\n")

				// Add data rows
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

				formattedResult = sb.String()
			}
		default:
			// For other types, use standard JSON marshaling
			jsonData, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				b.logger.Printf("Failed to marshal result: %v", err)
				return nil, fmt.Errorf("failed to marshal result: %w", err)
			}
			formattedResult = string(jsonData)
		}

		b.logger.Printf("Tool execution successful: %s", formattedResult)
		results = append(results, map[string]interface{}{
			"tool_call_id": call.ID,
			"output":       formattedResult,
		})
	}

	return results, nil
}

// Close cleans up resources
func (b *Bridge) Close() error {
	b.cancel()

	var errs []error

	if err := b.dbTool.Close(); err != nil {
		errs = append(errs, fmt.Errorf("database tool: %w", err))
	}

	// Clear maps and slices
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
	b.logger.Printf("Generating LLM response for message: %s", msg)
	resp, err := b.llmClient.GenerateResponse(msg)
	if err != nil {
		b.logger.Printf("LLM response generation failed: %v", err)
		return nil, err
	}

	b.logger.Printf("LLM response generated: %+v", resp)
	return resp, nil
}

func (b *Bridge) generateLLMResponseWithTools(toolResults []map[string]interface{}) (*types.LLMResponse, error) {
	b.logger.Printf("Continuing LLM conversation with tool results: %+v", toolResults)
	resp, err := b.llmClient.ContinueWithToolResults(toolResults)
	if err != nil {
		b.logger.Printf("LLM continuation failed: %v", err)
		return nil, err
	}

	b.logger.Printf("LLM continuation successful: %+v", resp)
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
