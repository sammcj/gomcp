// tools/filesystem.go
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// FileSystemTool provides file system operations
type FileSystemTool struct {
	basePath string
}

// NewFileSystemTool creates a new file system tool
func NewFileSystemTool(basePath string) (*FileSystemTool, error) {
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("invalid base path: %w", err)
	}

	return &FileSystemTool{basePath: absPath}, nil
}

// GetToolSpec returns the MCP tool specification
func (t *FileSystemTool) GetToolSpec() mcp.Tool {
	return mcp.Tool{
		Name:        "filesystem",
		Description: "Perform file system operations within a specified directory",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{"list", "read", "exists", "info"},
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Relative path within the base directory",
				},
			},
		},
	}
}

// Execute handles file system operations
func (t *FileSystemTool) Execute(params map[string]interface{}) (interface{}, error) {
	operation, _ := params["operation"].(string)
	path, _ := params["path"].(string)

	// Validate and clean path
	fullPath := filepath.Join(t.basePath, path)
	if !strings.HasPrefix(fullPath, t.basePath) {
		return nil, fmt.Errorf("path is outside base directory")
	}

	switch operation {
	case "list":
		return t.list(fullPath)
	case "read":
		return t.read(fullPath)
	case "exists":
		return t.exists(fullPath)
	case "info":
		return t.info(fullPath)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

func (t *FileSystemTool) list(path string) (interface{}, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []map[string]interface{}
	for _, entry := range entries {
		files = append(files, map[string]interface{}{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
			"size":  func() int64 {
				info, err := entry.Info()
				if err != nil {
					return 0
				}
				return info.Size()
			}(),
		})
	}
	return files, nil
}

func (t *FileSystemTool) read(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (t *FileSystemTool) exists(path string) (interface{}, error) {
	_, err := os.Stat(path)
	return err == nil, nil
}

func (t *FileSystemTool) info(path string) (interface{}, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":     info.Name(),
		"size":     info.Size(),
		"mode":     info.Mode().String(),
		"modTime":  info.ModTime(),
		"isDir":    info.IsDir(),
	}, nil
}

