// types/errors.go
package types

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfig indicates a configuration error
	ErrInvalidConfig = errors.New("invalid configuration")

	// ErrBridgeInit indicates bridge initialization failed
	ErrBridgeInit = errors.New("bridge initialization failed")

	// ErrLLMResponse indicates an invalid LLM response
	ErrLLMResponse = errors.New("invalid LLM response")

	// ErrToolExecution indicates a tool execution failure
	ErrToolExecution = errors.New("tool execution failed")

	// ErrDatabaseQuery indicates a database query error
	ErrDatabaseQuery = errors.New("database query failed")
)

// ConfigError wraps configuration-related errors
type ConfigError struct {
	Field   string
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("configuration error in %s: %s: %v", e.Field, e.Message, e.Err)
	}
	return fmt.Sprintf("configuration error in %s: %s", e.Field, e.Message)
}

func (e *ConfigError) Unwrap() error {
	return ErrInvalidConfig
}

// BridgeError wraps bridge-related errors
type BridgeError struct {
	Operation string
	Message   string
	Err       error
}

func (e *BridgeError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("bridge error during %s: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("bridge error during %s: %s", e.Operation, e.Message)
}

func (e *BridgeError) Unwrap() error {
	return ErrBridgeInit
}

// LLMError wraps LLM-related errors
type LLMError struct {
	Operation string
	Message   string
	Response  *LLMResponse
	Err       error
}

func (e *LLMError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("LLM error during %s: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("LLM error during %s: %s", e.Operation, e.Message)
}

func (e *LLMError) Unwrap() error {
	return ErrLLMResponse
}

// ToolError wraps tool-related errors
type ToolError struct {
	Tool    string
	Message string
	Err     error
}

func (e *ToolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("tool error in %s: %s: %v", e.Tool, e.Message, e.Err)
	}
	return fmt.Sprintf("tool error in %s: %s", e.Tool, e.Message)
}

func (e *ToolError) Unwrap() error {
	return ErrToolExecution
}

// DatabaseError wraps database-related errors
type DatabaseError struct {
	Operation string
	Query     string
	Message   string
	Err       error
}

func (e *DatabaseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("database error during %s: %s: %v", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("database error during %s: %s", e.Operation, e.Message)
}

func (e *DatabaseError) Unwrap() error {
	return ErrDatabaseQuery
}
