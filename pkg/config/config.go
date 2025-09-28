package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds all configuration options for the money CLI
type Config struct {
	// MoneyDir is the directory where money data is stored
	MoneyDir string

	// LLM configuration
	LLMPromptCmd  string
	LLMBatchSize  int

	// Default values
	DefaultLLMPromptCmd  string
	DefaultLLMBatchSize  int
	DefaultMoneyDirName  string
}

// New creates a new configuration instance with values from environment variables
func New() *Config {
	cfg := &Config{
		DefaultLLMPromptCmd:  "claude",
		DefaultLLMBatchSize:  10,
		DefaultMoneyDirName:  ".money",
	}

	cfg.loadFromEnvironment()
	return cfg
}

// loadFromEnvironment loads configuration from environment variables
func (c *Config) loadFromEnvironment() {
	// Money directory
	c.MoneyDir = c.getMoneyDir()

	// LLM configuration
	c.LLMPromptCmd = c.getLLMPromptCmd()
	c.LLMBatchSize = c.getLLMBatchSize()
}

// getMoneyDir returns the money directory path
func (c *Config) getMoneyDir() string {
	if dir := os.Getenv("MONEY_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, c.DefaultMoneyDirName)
}

// getLLMPromptCmd returns the LLM prompt command
func (c *Config) getLLMPromptCmd() string {
	if cmd := os.Getenv("LLM_PROMPT_CMD"); cmd != "" {
		return cmd
	}
	return c.DefaultLLMPromptCmd
}

// getLLMBatchSize returns the LLM batch size
func (c *Config) getLLMBatchSize() int {
	if batchSizeStr := os.Getenv("LLM_BATCH_SIZE"); batchSizeStr != "" {
		if batchSize, err := strconv.Atoi(batchSizeStr); err == nil && batchSize > 0 {
			return batchSize
		}
	}
	return c.DefaultLLMBatchSize
}

// SetMoneyDir updates the money directory path
func (c *Config) SetMoneyDir(dir string) {
	c.MoneyDir = dir
}

// SetLLMPromptCmd updates the LLM prompt command
func (c *Config) SetLLMPromptCmd(cmd string) {
	c.LLMPromptCmd = cmd
}

// SetLLMBatchSize updates the LLM batch size
func (c *Config) SetLLMBatchSize(size int) {
	c.LLMBatchSize = size
}

// ToEnvironmentVars returns a map of environment variables that can be set
func (c *Config) ToEnvironmentVars() map[string]string {
	vars := make(map[string]string)

	if c.MoneyDir != "" {
		vars["MONEY_DIR"] = c.MoneyDir
	}

	if c.LLMPromptCmd != c.DefaultLLMPromptCmd {
		vars["LLM_PROMPT_CMD"] = c.LLMPromptCmd
	}

	if c.LLMBatchSize != c.DefaultLLMBatchSize {
		vars["LLM_BATCH_SIZE"] = strconv.Itoa(c.LLMBatchSize)
	}

	return vars
}

// GetBashrcExports returns bash export statements for non-default configurations
func (c *Config) GetBashrcExports() []string {
	var exports []string

	// Only export non-default values
	if c.MoneyDir != "" {
		home, _ := os.UserHomeDir()
		defaultDir := filepath.Join(home, c.DefaultMoneyDirName)
		if c.MoneyDir != defaultDir {
			exports = append(exports, "export MONEY_DIR=\""+c.MoneyDir+"\"")
		}
	}

	if c.LLMPromptCmd != c.DefaultLLMPromptCmd {
		exports = append(exports, "export LLM_PROMPT_CMD=\""+c.LLMPromptCmd+"\"")
	}

	if c.LLMBatchSize != c.DefaultLLMBatchSize {
		exports = append(exports, "export LLM_BATCH_SIZE=\""+strconv.Itoa(c.LLMBatchSize)+"\"")
	}

	return exports
}

// DBPath returns the full path to the database file
func (c *Config) DBPath() string {
	return filepath.Join(c.MoneyDir, "money.db")
}

// EnsureMoneyDir creates the money directory if it doesn't exist
func (c *Config) EnsureMoneyDir() error {
	return os.MkdirAll(c.MoneyDir, 0755)
}