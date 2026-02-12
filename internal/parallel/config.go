package parallel

import (
	"os"
	"strconv"
)

type Config struct {
	EnableParallel      bool
	MaxWorkers          int
	SetupCommandWorkers int
	PackageQueryWorkers int
}

func DefaultConfig() *Config {
	return &Config{
		EnableParallel:      true,
		MaxWorkers:          4,
		SetupCommandWorkers: 3,
		PackageQueryWorkers: 5,
	}
}

func LoadConfig() *Config {
	config := DefaultConfig()

	if os.Getenv("CODERAFT_DISABLE_PARALLEL") == "true" {
		config.EnableParallel = false
		return config
	}

	if maxWorkers := os.Getenv("CODERAFT_MAX_WORKERS"); maxWorkers != "" {
		if val, err := strconv.Atoi(maxWorkers); err == nil && val > 0 {
			config.MaxWorkers = val
		}
	}

	if setupWorkers := os.Getenv("CODERAFT_SETUP_WORKERS"); setupWorkers != "" {
		if val, err := strconv.Atoi(setupWorkers); err == nil && val > 0 {
			config.SetupCommandWorkers = val
		}
	}

	if queryWorkers := os.Getenv("CODERAFT_QUERY_WORKERS"); queryWorkers != "" {
		if val, err := strconv.Atoi(queryWorkers); err == nil && val > 0 {
			config.PackageQueryWorkers = val
		}
	}

	return config
}
