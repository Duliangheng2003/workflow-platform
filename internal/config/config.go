package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the workflow platform.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	LLM      LLMConfig      `yaml:"llm"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DatabaseConfig holds MySQL connection settings.
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// DSN returns the MySQL Data Source Name string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.Database)
}

// LLMConfig holds pre-configured LLM provider profiles.
// API keys are stored here on the server side — never exposed to frontend.
type LLMConfig struct {
	Profiles []LLMProfile `yaml:"profiles"`
}

// LLMProfile defines a named LLM configuration that workflow templates can reference.
type LLMProfile struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
}

// LookupProfile returns the LLM profile with the given name, or an error.
func (l LLMConfig) LookupProfile(name string) (*LLMProfile, error) {
	for i := range l.Profiles {
		if l.Profiles[i].Name == name {
			return &l.Profiles[i], nil
		}
	}
	return nil, fmt.Errorf("LLM profile not found: %s", name)
}

// Load reads a YAML config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}

	return &cfg, nil
}