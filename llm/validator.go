package llm

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sammcj/gomcp/types"
)

// Validator validates LLM responses and tool calls
type Validator struct {
	tools map[string]mcp.Tool
}

// NewValidator creates a new validator with the given tools
func NewValidator(tools []mcp.Tool) *Validator {
	toolMap := make(map[string]mcp.Tool)
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}
	return &Validator{tools: toolMap}
}

// ValidateResponse validates an LLM response
func (v *Validator) ValidateResponse(resp *types.LLMResponse) error {
	if resp == nil {
		return fmt.Errorf("response is nil")
	}

	// Validate tool calls if present
	for _, call := range resp.ToolCalls {
		if err := v.validateToolCall(call); err != nil {
			return fmt.Errorf("invalid tool call: %w", err)
		}
	}

	return nil
}

// validateToolCall validates a tool call
func (v *Validator) validateToolCall(call types.ToolCall) error {
	// Check if tool exists
	tool, ok := v.tools[call.Function.Name]
	if !ok {
		return fmt.Errorf("unknown tool: %s", call.Function.Name)
	}

	// Validate arguments against schema
	if err := v.validateArguments(call.Function.Arguments, tool.InputSchema); err != nil {
		return fmt.Errorf("invalid arguments for tool %s: %w", call.Function.Name, err)
	}

	return nil
}

// validateArguments validates tool arguments against a schema
func (v *Validator) validateArguments(args map[string]interface{}, schema mcp.ToolInputSchema) error {
	// Check required fields
	for _, required := range schema.Required {
		if _, ok := args[required]; !ok {
			return fmt.Errorf("missing required field: %s", required)
		}
	}

	// Validate properties
	for name, value := range args {
		propSchema, ok := schema.Properties[name]
		if !ok {
			return fmt.Errorf("unknown property: %s", name)
		}

		propType, ok := propSchema.(map[string]interface{})["type"].(string)
		if !ok {
			return fmt.Errorf("invalid property schema for %s", name)
		}

		if err := v.validateType(value, propType); err != nil {
			return fmt.Errorf("invalid value for %s: %w", name, err)
		}
	}

	return nil
}

// validateType validates a value against a JSON Schema type
func (v *Validator) validateType(value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "number":
		switch value.(type) {
		case float64, float32, int, int64, int32:
			// Valid numeric types
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
	default:
		return fmt.Errorf("unsupported type: %s", expectedType)
	}

	return nil
}
