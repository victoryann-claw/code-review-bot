package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration
type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	GitHub struct {
		Token            string `yaml:"token"`
		WebhookSecret    string `yaml:"webhook_secret"`
	} `yaml:"github"`
	LLM struct {
		Provider   string  `yaml:"provider"`
		APIKey     string  `yaml:"api_key"`
		Model      string  `yaml:"model"`
		BaseURL    string  `yaml:"base_url"`
		Temperature float64 `yaml:"temperature"`
		MaxTokens  int     `yaml:"max_tokens"`
	} `yaml:"llm"`
	Debug bool `yaml:"debug"`
}

// Global configuration instance
var globalConfig *Config

// LoadConfig loads configuration from YAML file
func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	globalConfig = &Config{}
	if err := yaml.Unmarshal(data, globalConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables if set
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		globalConfig.GitHub.Token = token
	}
	if secret := os.Getenv("GITHUB_WEBHOOK_SECRET"); secret != "" {
		globalConfig.GitHub.WebhookSecret = secret
	}
	if provider := os.Getenv("LLM_PROVIDER"); provider != "" {
		globalConfig.LLM.Provider = provider
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		globalConfig.LLM.APIKey = apiKey
	}
	if apiKey := os.Getenv("MINIMAX_API_KEY"); apiKey != "" {
		globalConfig.LLM.APIKey = apiKey
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		globalConfig.LLM.Model = model
	}
	if model := os.Getenv("MINIMAX_MODEL"); model != "" {
		globalConfig.LLM.Model = model
	}
	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		globalConfig.LLM.BaseURL = baseURL
	}
	if debug := os.Getenv("DEBUG"); debug == "true" {
		globalConfig.Debug = true
	}

	return nil
}

// GetConfig returns the current configuration
func GetConfig() *Config {
	return globalConfig
}
