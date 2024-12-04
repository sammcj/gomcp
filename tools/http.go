// tools/http.go
package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// HTTPTool provides HTTP request capabilities
type HTTPTool struct {
	client    *http.Client
	allowList []string
}

// NewHTTPTool creates a new HTTP tool with domain allowlist
func NewHTTPTool(allowList []string, timeout time.Duration) *HTTPTool {
	return &HTTPTool{
		client: &http.Client{
			Timeout: timeout,
		},
		allowList: allowList,
	}
}

// GetToolSpec returns the MCP tool specification
func (t *HTTPTool) GetToolSpec() mcp.Tool {
	return mcp.Tool{
		Name:        "http_request",
		Description: "Make HTTP requests to allowed domains",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"method": map[string]interface{}{
					"type": "string",
					"enum": []string{"GET", "POST", "PUT", "DELETE"},
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL to request",
				},
				"headers": map[string]interface{}{
					"type": "object",
					"additionalProperties": map[string]interface{}{
						"type": "string",
					},
				},
				"body": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
}

// Execute handles HTTP requests
func (t *HTTPTool) Execute(params map[string]interface{}) (interface{}, error) {
	method, _ := params["method"].(string)
	url, _ := params["url"].(string)
	headers, _ := params["headers"].(map[string]interface{})
	body, _ := params["body"].(string)

	// Check URL against allowlist
	allowed := false
	for _, domain := range t.allowList {
		if strings.HasPrefix(url, domain) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("domain not in allowlist")
	}

	// Create request
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Add headers
	for k, v := range headers {
		if strVal, ok := v.(string); ok {
			req.Header.Set(k, strVal)
		}
	}

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status":  resp.StatusCode,
		"headers": resp.Header,
		"body":    string(respBody),
	}, nil
}
