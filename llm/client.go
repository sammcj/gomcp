package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/gomcp/types"
)

// Client manages communication with the Ollama API
type Client struct {
	endpoint     string
	model        string
	systemPrompt string
	httpClient   *http.Client
	tools        []mcp.Tool
	logger       *log.Logger
}

// Request represents a request to the Ollama API
type Request struct {
	Model    string          `json:"model"`
	Messages []types.Message `json:"messages"`
	Stream   bool            `json:"stream"`
	Tools    []interface{}   `json:"tools,omitempty"`
}

// Response represents a response from the Ollama API
type Response struct {
	Model   string `json:"model"`
	Message struct {
		Role      string           `json:"role"`
		Content   string           `json:"content"`
		ToolCalls []types.ToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
}

// New creates a new Ollama client
func New(endpoint, model, systemPrompt string) *Client {
	return &Client{
		endpoint:     endpoint,
		model:        model,
		systemPrompt: systemPrompt,
		httpClient:   &http.Client{},
		logger:       log.Default(),
	}
}

// SetTools configures the available tools for the model
func (c *Client) SetTools(tools []mcp.Tool) error {
	c.tools = tools
	return nil
}

// GenerateResponse sends a message to the model and gets its response
func (c *Client) GenerateResponse(msg string) (*types.LLMResponse, error) {
	// Convert MCP tools to Ollama format
	ollamaTools := c.convertTools()

	// Build messages array with system prompt
	messages := []types.Message{
		{Role: "system", Content: c.systemPrompt},
		{Role: "user", Content: msg},
	}

	// Create request
	req := Request{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Tools:    ollamaTools,
	}

	// Send request
	resp, err := c.sendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// ContinueWithToolResults continues the conversation after tool execution
func (c *Client) ContinueWithToolResults(toolResults []map[string]interface{}) (*types.LLMResponse, error) {
	var messages []types.Message
	if c.systemPrompt != "" {
		messages = append(messages, types.Message{
			Role:    "system",
			Content: c.systemPrompt,
		})
	}

	// Add each tool result as a separate message
	for _, result := range toolResults {
		toolCallID := result["tool_call_id"].(string)
		output := result["output"].(string)
		messages = append(messages, types.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Tool result for call %s: %s", toolCallID, output),
		})
	}

	// Create request
	req := Request{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Tools:    c.convertTools(),
	}

	// Send request
	return c.sendRequest(req)
}

// convertTools converts MCP tools to Ollama format
func (c *Client) convertTools() []interface{} {
	var ollamaTools []interface{}

	for _, tool := range c.tools {
		ollamaTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        sanitizeToolName(tool.Name),
				"description": tool.Description,
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": tool.InputSchema.Properties,
					"required":   tool.InputSchema.Required,
				},
			},
		}
		ollamaTools = append(ollamaTools, ollamaTool)
	}

	return ollamaTools
}

// sendRequest sends a request to the Ollama API
func (c *Client) sendRequest(req Request) (*types.LLMResponse, error) {

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use the full endpoint URL
	endpoint := fmt.Sprintf("%s/chat", c.endpoint)

	// c.logger.Printf("Sending request to Ollama endpoint: %s", endpoint)
	// c.logger.Printf("Request data: %s", string(data))

	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Printf("Received response from Ollama: %s", string(body))

	var ollamaResp Response
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to types.LLMResponse
	result := &types.LLMResponse{
		Content:   ollamaResp.Message.Content,
		ToolCalls: ollamaResp.Message.ToolCalls,
	}

	c.logger.Printf("Converted response: %+v", result)
	return result, nil
}

// sanitizeToolName converts a tool name to a format compatible with Ollama
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
