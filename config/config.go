// config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigDir  = ".config/gomcp"
	defaultConfigFile = "config.yaml"
)

// MCPServerConfig holds configuration for a single MCP server
type MCPServerConfig struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Arguments []string          `yaml:"arguments"`
	Env       map[string]string `yaml:"env,omitempty"`
}

// Config holds the complete configuration for the bridge
type Config struct {
	LLM struct {
		Model        string `yaml:"model"`
		Endpoint     string `yaml:"endpoint"`
		APIKey       string `yaml:"api_key"`
		SystemPrompt string `yaml:"system_prompt"`
	} `yaml:"llm"`

	MCPServers []MCPServerConfig `yaml:"mcp_servers"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`

	Server struct {
		Enable bool   `yaml:"enable"`
		Host   string `yaml:"host"`
		Port   int    `yaml:"port"`
	} `yaml:"server"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	cfg := &Config{}

	// LLM defaults
	cfg.LLM.Model = "qwen2.5-coder-7b-instruct-128k:q6_k"
	cfg.LLM.Endpoint = "http://localhost:11434/api"
	cfg.LLM.SystemPrompt = `You are a helpful assistant with access to various tools.

[Tools]
When using the database tool:
1. Use exact column names from the schema
2. Write valid SQL queries
3. Remember this is SQLite

When using bybit tools:
1. All amounts are in USD
2. Follow proper position sizing and risk management
3. Always verify order details before execution`

	// Default MCP servers
	cfg.MCPServers = []MCPServerConfig{
		{
			Name:      "sqlite",
			Command:   "uvx",
			Arguments: []string{"mcp-server-sqlite", "--db-path", "test.db"},
		},
		{
			Name:    "bybit",
			Command: "/bin/sh",
			Arguments: []string{
				"-c",
				"pnpm run serve", // User must configure the correct path in their config file
			},
			Env: map[string]string{
				"BYBIT_API_KEY":       "", // Add your Bybit API **READ ONLY** key here
				"BYBIT_API_SECRET":    "", // Add your Bybit API **READ ONLY** secret here
				"BYBIT_USE_TESTNET":   "true",
			},
		},
	}

	// Database defaults
	cfg.Database.Path = "test.db"

	// Logging defaults
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "text"

	// Server defaults - set to false by default
	cfg.Server.Enable = false
	cfg.Server.Host = "localhost"
	cfg.Server.Port = 8080

	return cfg
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, defaultConfigDir)
	return filepath.Join(configDir, defaultConfigFile), nil
}

// LoadOrCreate loads the config file if it exists, or creates a default one if it doesn't
func LoadOrCreate() (*Config, bool, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, false, err
	}

	// Check if config directory exists
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, false, fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		cfg := DefaultConfig()
		if err := cfg.Save(); err != nil {
			return nil, false, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, true, nil
	}

	// Load existing config
	cfg, err := Load(configPath)
	return cfg, false, err
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with default config to ensure all fields have values
	cfg := DefaultConfig()

	// Unmarshal into a temporary map to check if server.enable is explicitly set
	var tempConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &tempConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Now unmarshal into the actual config struct
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Check if server.enable was explicitly set in the config file
	if serverConfig, ok := tempConfig["server"].(map[string]interface{}); ok {
		if enable, ok := serverConfig["enable"].(bool); ok {
			cfg.Server.Enable = enable
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to disk
func (c *Config) Save() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// validate checks that required fields are present and valid
func (c *Config) validate() error {
	// Required LLM fields
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model is required")
	}
	if c.LLM.Endpoint == "" {
		return fmt.Errorf("llm.endpoint is required")
	}

	// Required MCP fields
	if len(c.MCPServers) == 0 {
		return fmt.Errorf("at least one MCP server configuration is required")
	}
	for i, server := range c.MCPServers {
		if server.Name == "" {
			return fmt.Errorf("mcp_servers[%d].name is required", i)
		}
		if server.Command == "" {
			return fmt.Errorf("mcp_servers[%d].command is required", i)
		}
	}

	// Required Database fields
	if c.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}

	return nil
}
