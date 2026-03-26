package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigFile = "garance.json"

type Config struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Engine  string `json:"engine"` // "postgresql"
}

func Init(dir, name string) error {
	// Create garance.json
	config := Config{
		Name:    name,
		Version: "0.1.0",
		Engine:  "postgresql",
	}

	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, ConfigFile), configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", ConfigFile, err)
	}

	// Create garance.schema.ts
	schemaContent := DefaultSchemaTemplate()
	if err := os.WriteFile(filepath.Join(dir, "garance.schema.ts"), []byte(schemaContent), 0644); err != nil {
		return fmt.Errorf("failed to write garance.schema.ts: %w", err)
	}

	// Create seed file
	seedDir := filepath.Join(dir, "seeds")
	if err := os.MkdirAll(seedDir, 0755); err != nil {
		return fmt.Errorf("failed to create seeds directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "seed.sql"), []byte(DefaultSeedTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to write seed.sql: %w", err)
	}

	// Create migrations directory
	if err := os.MkdirAll(filepath.Join(dir, "migrations"), 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Create package.json if it doesn't exist
	packageJSONPath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		if err := os.WriteFile(packageJSONPath, []byte(DefaultPackageJSONTemplate()), 0644); err != nil {
			return fmt.Errorf("failed to write package.json: %w", err)
		}
	}

	// Create .env.local
	envContent := DefaultEnvTemplate()
	if err := os.WriteFile(filepath.Join(dir, ".env.local"), []byte(envContent), 0644); err != nil {
		return fmt.Errorf("failed to write .env.local: %w", err)
	}

	return nil
}

func LoadConfig(dir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", ConfigFile, err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", ConfigFile, err)
	}
	return &config, nil
}

func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ConfigFile))
	return err == nil
}
