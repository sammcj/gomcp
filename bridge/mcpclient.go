package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

type MCPClient struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	logger   *log.Logger
	mu       sync.Mutex
	nextID   int64
	respChan chan []byte
}

// isDevelopmentModeWarning checks if a message is a development mode warning
func isDevelopmentModeWarning(msg string) bool {
	return strings.Contains(msg, "Running in development mode")
}

func NewMCPClient(command string, args []string, logger *log.Logger) (*MCPClient, error) {
	logger.Printf("Creating new MCP client with command: %s %v", command, args)

	// Create a temporary file for stderr output
	stderrFile, err := os.CreateTemp("", "mcp-stderr-*.log")
	if err != nil {
		logger.Printf("Failed to create stderr file: %v", err)
		return nil, fmt.Errorf("failed to create stderr file: %w", err)
	}
	defer stderrFile.Close()

	cmd := exec.Command(command, args...)
	cmd.Stderr = stderrFile

	// Set up environment
	env := os.Environ()
	cmd.Env = append(env,
		"RUST_LOG=debug",
		"RUST_BACKTRACE=1",
		"PYTHONUNBUFFERED=1",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Printf("Failed to create stdin pipe: %v", err)
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Printf("Failed to create stdout pipe: %v", err)
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	logger.Printf("Starting MCP server process...")
	if err := cmd.Start(); err != nil {
		logger.Printf("Failed to start MCP server: %v", err)
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	client := &MCPClient{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		logger:   logger,
		nextID:   1,
		respChan: make(chan []byte, 1),
	}

	// Start reading responses in a goroutine
	go client.readResponses()

	// Wait for server to start and check stderr for any startup errors
	time.Sleep(2 * time.Second)
	stderrFile.Seek(0, 0)
	stderrBytes, err := io.ReadAll(stderrFile)
	if err != nil {
		logger.Printf("Failed to read stderr: %v", err)
	} else if len(stderrBytes) > 0 {
		// Only log stderr if it's not a development mode warning
		if !isDevelopmentModeWarning(string(stderrBytes)) {
			logger.Printf("MCP server stderr output: %s", string(stderrBytes))
		}
	}

	// Send a test request to verify server is working
	testReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      client.nextID,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	client.nextID++

	if err := client.sendRequest(testReq); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to send test request: %w", err)
	}

	// Wait for test response
	select {
	case respBytes, ok := <-client.respChan:
		if !ok {
			client.Close()
			return nil, fmt.Errorf("response channel closed during test")
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to unmarshal test response: %w", err)
		}
		logger.Printf("Test response received: %+v", resp)

	case <-time.After(5 * time.Second):
		client.Close()
		return nil, fmt.Errorf("timeout waiting for test response")
	}

	logger.Printf("MCP server process started with PID: %d", cmd.Process.Pid)
	return client, nil
}

func (c *MCPClient) readResponses() {
	reader := bufio.NewReader(c.stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				c.logger.Printf("Error reading response: %v", err)
			}
			close(c.respChan)
			return
		}

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Try to parse as JSON
		var msg map[string]interface{}
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip non-JSON lines (e.g., pnpm startup messages)
			continue
		}

		// Handle notifications separately
		if _, hasMethod := msg["method"]; hasMethod {
			if method, ok := msg["method"].(string); ok && method == "notify" {
				if params, ok := msg["params"].(map[string]interface{}); ok {
					// Only log notifications that aren't development mode warnings
					if !isDevelopmentModeWarning(fmt.Sprintf("%v", params)) {
						c.logger.Printf("Notification received: %+v", params)
					}
				}
				continue
			}
		}

		// Only log and forward actual responses
		if _, hasResult := msg["result"]; hasResult {
			c.logger.Printf("Response received: %s", string(line))
			c.respChan <- line
		}
	}
}

func (c *MCPClient) Initialize(ctx context.Context, req mcp.InitializeRequest) (*mcp.InitializeResult, error) {
	c.logger.Printf("Sending initialize request: %+v", req)

	// Create JSON-RPC request with correct method name
	rpcReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "1.0.0",
			"capabilities": map[string]interface{}{
				"experimental": map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "gomcp",
				"version": "0.1.0",
			},
		},
	}
	c.nextID++

	// Send request
	if err := c.sendRequest(rpcReq); err != nil {
		c.logger.Printf("Failed to send initialize request: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response with timeout
	select {
	case respBytes, ok := <-c.respChan:
		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			c.logger.Printf("Failed to unmarshal response: %v", err)
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		c.logger.Printf("Received initialize response: %+v", resp)

		// Parse response
		result := &mcp.InitializeResult{}
		if resultData, ok := resp["result"].(map[string]interface{}); ok {
			if capabilities, ok := resultData["capabilities"].(map[string]interface{}); ok {
				if exp, ok := capabilities["experimental"].(map[string]interface{}); ok {
					result.Capabilities.Experimental = exp
				}
			}
		}
		return result, nil

	case <-time.After(10 * time.Second):
		c.logger.Printf("Timeout waiting for initialize response")
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (c *MCPClient) ListTools(ctx context.Context, req mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	c.logger.Printf("Sending list tools request")

	// Create JSON-RPC request
	rpcReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	c.nextID++

	// Send request
	if err := c.sendRequest(rpcReq); err != nil {
		c.logger.Printf("Failed to send list tools request: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response with timeout
	select {
	case respBytes, ok := <-c.respChan:
		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			c.logger.Printf("Failed to unmarshal response: %v", err)
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		c.logger.Printf("Received list tools response: %+v", resp)

		// Parse response
		result := &mcp.ListToolsResult{}
		if resultData, ok := resp["result"].(map[string]interface{}); ok {
			if tools, ok := resultData["tools"].([]interface{}); ok {
				for _, tool := range tools {
					if toolMap, ok := tool.(map[string]interface{}); ok {
						name, _ := toolMap["name"].(string)
						desc, _ := toolMap["description"].(string)

						// Convert schema to ToolInputSchema
						var schema mcp.ToolInputSchema
						if schemaMap, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
							schemaBytes, err := json.Marshal(schemaMap)
							if err != nil {
								c.logger.Printf("Failed to marshal schema: %v", err)
								continue
							}
							if err := json.Unmarshal(schemaBytes, &schema); err != nil {
								c.logger.Printf("Failed to unmarshal schema: %v", err)
								continue
							}
						}

						result.Tools = append(result.Tools, mcp.Tool{
							Name:        name,
							Description: desc,
							InputSchema: schema,
						})
					}
				}
			}
		}
		return result, nil

	case <-time.After(10 * time.Second):
		c.logger.Printf("Timeout waiting for list tools response")
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (c *MCPClient) CallTool(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c.logger.Printf("Sending call tool request: %+v", req)

	// Create JSON-RPC request
	rpcReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.nextID,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      req.Params.Name,
			"arguments": req.Params.Arguments,
		},
	}
	c.nextID++

	// Send request
	if err := c.sendRequest(rpcReq); err != nil {
		c.logger.Printf("Failed to send call tool request: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response with timeout
	select {
	case respBytes, ok := <-c.respChan:
		if !ok {
			return nil, fmt.Errorf("response channel closed")
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			c.logger.Printf("Failed to unmarshal response: %v", err)
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		c.logger.Printf("Received call tool response: %+v", resp)

		// Parse response
		result := &mcp.CallToolResult{}
		if resultData, ok := resp["result"].(map[string]interface{}); ok {
			if content, ok := resultData["content"].([]interface{}); ok {
				for _, item := range content {
					if textContent, ok := item.(map[string]interface{}); ok {
						if text, ok := textContent["text"].(string); ok {
							result.Content = append(result.Content, mcp.TextContent{
								Text: text,
							})
						}
					}
				}
			}
		}
		return result, nil

	case <-time.After(10 * time.Second):
		c.logger.Printf("Timeout waiting for call tool response")
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (c *MCPClient) Close() error {
	c.logger.Println("Closing MCP client...")

	if err := c.stdin.Close(); err != nil {
		c.logger.Printf("Failed to close stdin: %v", err)
		return fmt.Errorf("failed to close stdin: %w", err)
	}

	if err := c.stdout.Close(); err != nil {
		c.logger.Printf("Failed to close stdout: %v", err)
		return fmt.Errorf("failed to close stdout: %w", err)
	}

	if err := c.cmd.Process.Kill(); err != nil {
		c.logger.Printf("Failed to kill process: %v", err)
		return fmt.Errorf("failed to kill process: %w", err)
	}

	if err := c.cmd.Wait(); err != nil {
		c.logger.Printf("Failed to wait for process: %v", err)
		return fmt.Errorf("failed to wait for process: %w", err)
	}

	c.logger.Println("MCP client closed successfully")
	return nil
}

func (c *MCPClient) sendRequest(req interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		c.logger.Printf("Failed to marshal request: %v", err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Printf("Sending raw request: %s", string(data))

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		c.logger.Printf("Failed to write request: %v", err)
		return fmt.Errorf("failed to write request: %w", err)
	}

	return nil
}
