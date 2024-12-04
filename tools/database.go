// tools/database.go
package tools

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	_ "github.com/mattn/go-sqlite3"
)

// DatabaseTool handles database operations
type DatabaseTool struct {
	db      *sql.DB
	schemas map[string]TableSchema
}

// TableSchema represents a database table schema
type TableSchema struct {
	Name        string
	Description string
	Columns     []ColumnSchema
}

// ColumnSchema represents a database column schema
type ColumnSchema struct {
	Name        string
	Type        string
	Description string
}

// NewDatabaseTool creates a new database tool instance
func NewDatabaseTool(dbPath string) (*DatabaseTool, error) {
    // Open database connection
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Create tool instance
    tool := &DatabaseTool{
        db:      db,
        schemas: make(map[string]TableSchema),
    }

    // Load schemas
    if err := tool.loadSchemas(); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to load schemas: %w", err)
    }

    return tool, nil
}

// loadSchemas reads the database schema
func (t *DatabaseTool) loadSchemas() error {
    // Get list of tables
    rows, err := t.db.Query(`
        SELECT name FROM sqlite_master
        WHERE type='table' AND name NOT LIKE 'sqlite_%'
    `)
    if err != nil {
        return fmt.Errorf("failed to list tables: %w", err)
    }
    defer rows.Close()

    // Process each table
    for rows.Next() {
        var tableName string
        if err := rows.Scan(&tableName); err != nil {
            return fmt.Errorf("failed to scan table name: %w", err)
        }

        // Get table schema
        schema, err := t.getTableSchema(tableName)
        if err != nil {
            return fmt.Errorf("failed to get schema for %s: %w", tableName, err)
        }

        t.schemas[tableName] = schema
    }

    return nil
}

// getTableSchema reads the schema for a specific table
func (t *DatabaseTool) getTableSchema(tableName string) (TableSchema, error) {
    schema := TableSchema{
        Name: tableName,
    }

    // Get column information
    rows, err := t.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
    if err != nil {
        return schema, fmt.Errorf("failed to get table info: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var cid int
        var name, typ string
        var notNull, pk int
        var dflt_value interface{}
        if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt_value, &pk); err != nil {
            return schema, fmt.Errorf("failed to scan column info: %w", err)
        }

        schema.Columns = append(schema.Columns, ColumnSchema{
            Name: name,
            Type: typ,
        })
    }

    return schema, nil
}

// GetToolSpec returns the MCP tool specification
func (t *DatabaseTool) GetToolSpec() mcp.Tool {
    // Build schema description
    var schemaDesc strings.Builder
    for _, schema := range t.schemas {
        schemaDesc.WriteString(fmt.Sprintf("\nTable %s:\n", schema.Name))
        for _, col := range schema.Columns {
            schemaDesc.WriteString(fmt.Sprintf("  - %s (%s)\n", col.Name, col.Type))
        }
    }

    return mcp.Tool{
        Name:        "query_database",
        Description: fmt.Sprintf("Execute SQL queries against the database. Available schemas: %s", schemaDesc.String()),
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "query": map[string]interface{}{
                    "type":        "string",
                    "description": "SQL query to execute",
                },
            },
        },
    }
}

// Execute runs a SQL query
func (t *DatabaseTool) Execute(params map[string]interface{}) (interface{}, error) {
    // Extract query
    query, ok := params["query"].(string)
    if !ok {
        return nil, fmt.Errorf("query parameter is required")
    }

    // Validate query
    if err := t.validateQuery(query); err != nil {
        return nil, fmt.Errorf("invalid query: %w", err)
    }

    // Execute query
    rows, err := t.db.Query(query)
    if err != nil {
        return nil, fmt.Errorf("failed to execute query: %w", err)
    }
    defer rows.Close()

    // Get column names
    columns, err := rows.Columns()
    if err != nil {
        return nil, fmt.Errorf("failed to get columns: %w", err)
    }

    // Prepare result
    var results []map[string]interface{}
    values := make([]interface{}, len(columns))
    scanArgs := make([]interface{}, len(columns))
    for i := range values {
        scanArgs[i] = &values[i]
    }

    // Process rows
    for rows.Next() {
        if err := rows.Scan(scanArgs...); err != nil {
            return nil, fmt.Errorf("failed to scan row: %w", err)
        }

        row := make(map[string]interface{})
        for i, col := range columns {
            val := values[i]
            if val != nil {
                // Convert []byte to string for better readability
                if b, ok := val.([]byte); ok {
                    row[col] = string(b)
                } else {
                    row[col] = val
                }
            }
        }
        results = append(results, row)
    }

    // Return results directly without JSON marshaling
    return results, nil
}

// validateQuery performs basic query validation
func (t *DatabaseTool) validateQuery(query string) error {
    lowerQuery := strings.ToLower(query)

    // Check for dangerous operations
    if strings.Contains(lowerQuery, "drop") ||
        strings.Contains(lowerQuery, "alter") ||
        strings.Contains(lowerQuery, "delete") ||
        strings.Contains(lowerQuery, "update") ||
        strings.Contains(lowerQuery, "insert") {
        return fmt.Errorf("only SELECT queries are allowed")
    }

    // Validate table names
    for tableName := range t.schemas {
        if strings.Contains(lowerQuery, strings.ToLower(tableName)) {
            return nil
        }
    }

    return fmt.Errorf("query must reference a valid table")
}

// Close releases database resources
func (t *DatabaseTool) Close() error {
    return t.db.Close()
}
