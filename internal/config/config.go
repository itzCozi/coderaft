package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ConfigManager struct {
	configPath string
}

func NewConfigManager() (*ConfigManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".coderaft")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	templatesDir := filepath.Join(configDir, "templates")
	_ = os.MkdirAll(templatesDir, 0755)

	configPath := filepath.Join(configDir, "config.json")
	return &ConfigManager{configPath: configPath}, nil
}

func NewConfigManagerWithPath(configDir string) (*ConfigManager, error) {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	templatesDir := filepath.Join(configDir, "templates")
	_ = os.MkdirAll(templatesDir, 0755)

	configPath := filepath.Join(configDir, "config.json")
	return &ConfigManager{configPath: configPath}, nil
}

func (cm *ConfigManager) Load() (*Config, error) {
	config := &Config{
		Projects: make(map[string]*Project),
		Settings: &GlobalSettings{
			DefaultBaseImage: "buildpack-deps:bookworm",
			AutoUpdate:       true,
			AutoStopOnExit:   true,
			AutoApplyLock:    true,
		},
	}

	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return config, nil
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) == 0 {
		return config, nil
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Settings == nil {
		config.Settings = &GlobalSettings{
			DefaultBaseImage: "buildpack-deps:bookworm",
			AutoUpdate:       true,
			AutoStopOnExit:   true,
			AutoApplyLock:    true,
		}
	}

	return config, nil
}

func (cm *ConfigManager) Save(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmpPath := cm.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	if err := os.Rename(tmpPath, cm.configPath); err != nil {

		os.Remove(tmpPath)
		if err := os.WriteFile(cm.configPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	return nil
}
