package config

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/lucasb-eyer/go-colorful"
)

const (
	configDir  = ".bluelink"
	configFile = "config.json"
)

// Config represents user configuration
type Config struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Color    string `json:"color"`
}

// LoadOrCreate loads existing config or creates a new one
func LoadOrCreate() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Try to load existing config
	cfg := &Config{}
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		return cfg, nil
	}

	// Config doesn't exist, create a new one
	return createNewConfig(configPath)
}

// createNewConfig prompts the user for information and creates a new config
func createNewConfig(configPath string) (*Config, error) {
	fmt.Print("Enter your name: ")
	var name string
	fmt.Scanln(&name)

	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	// Generate a random user ID
	rand.Seed(time.Now().UnixNano())
	userID := fmt.Sprintf("user_%d", rand.Intn(1000000))

	// Generate a random color
	color := generateRandomColor()

	cfg := &Config{
		UserID:   userID,
		Username: name,
		Color:    color,
	}

	// Save the config
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	return cfg, nil
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, configDir, configFile), nil
}

// generateRandomColor returns a random, visually pleasing terminal color
func generateRandomColor() string {
	// Create a random color that's not too dark
	c := colorful.Hsv(rand.Float64()*360.0, 0.7, 0.9)
	return c.Hex()
} 