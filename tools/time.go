// tools/time.go

package tools

import (
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// TimeTool provides time-related operations
type TimeTool struct{}

// GetToolSpec returns the MCP tool specification
func (t *TimeTool) GetToolSpec() mcp.Tool {
	return mcp.Tool{
		Name:        "time",
		Description: "Perform time-related operations",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{"now", "parse", "format", "compare"},
				},
				"timestamp": map[string]interface{}{
					"type": "string",
				},
				"format": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
}

// Execute handles time operations
func (t *TimeTool) Execute(params map[string]interface{}) (interface{}, error) {
	operation, _ := params["operation"].(string)
	timestamp, _ := params["timestamp"].(string)
	format, _ := params["format"].(string)

	switch operation {
	case "now":
		return time.Now(), nil

	case "parse":
		if format == "" {
			format = time.RFC3339
		}
		parsed, err := time.Parse(format, timestamp)
		if err != nil {
			return nil, err
		}
		return parsed, nil

	case "format":
		t, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, err
		}
		if format == "" {
			format = time.RFC3339
		}
		return t.Format(format), nil

	case "compare":
		t1, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, err
		}
		t2 := time.Now()
		diff := t2.Sub(t1)
		return map[string]interface{}{
			"before":     t1.Before(t2),
			"after":      t1.After(t2),
			"equal":      t1.Equal(t2),
			"difference": diff.String(),
		}, nil

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}
