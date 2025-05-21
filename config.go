package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey string `json:"api_key"`
}

func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	appConfigDir := filepath.Join(configDir, "gemini-cli")
	if err := os.MkdirAll(appConfigDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create app config directory %s: %w", appConfigDir, err)
	}
	return filepath.Join(appConfigDir, "config.json"), nil
}

func saveAPIKey(apiKey string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	config := Config{APIKey: apiKey}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}
	fmt.Printf("API key saved to %s\n", configPath)
	return nil
}

func loadAPIKey() (string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file not found at %s. Run 'set-config --key YOUR_KEY' first", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal config from %s: %w", configPath, err)
	}

	if config.APIKey == "" {
		return "", fmt.Errorf("API key not found in config file %s. Run 'set-config --key YOUR_KEY'", configPath)
	}
	return config.APIKey, nil
}
