package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/plaid/plaid-go/v27/plaid"
)

type Config struct {
	ClientID    string
	Secret      string
	Environment plaid.Environment
}

// SaveConfig saves the Plaid credentials to the config file located at ~/.config/money/credentials.json
func (c *Config) SaveConfig() error {
	// Create the directory if it doesn't exist
	dir := filepath.Join(os.Getenv("HOME"), ".config", "money")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	// Create the file if it doesn't exist
	file, err := os.Create(filepath.Join(dir, "credentials.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the credentials to the file
	enc := json.NewEncoder(file)
	err = enc.Encode(c)
	if err != nil {
		return err
	}

	return nil
}

// LoadConfig loads the Plaid credentials from the config file located at ~/.config/money/credentials.json
func LoadConfig() (*Config, error) {
	// Open the file
	file, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "money", "credentials.json"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Decode the credentials
	dec := json.NewDecoder(file)
	creds := Config{}
	err = dec.Decode(&creds)
	if err != nil {
		return nil, err
	}

	return &creds, nil
}
