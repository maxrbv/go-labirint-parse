package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Parser ParserConfig `yaml:"parser"`
	Logger LoggerConfig `yaml:"logger"`
}

type ParserConfig struct {
	BaseURL      string        `yaml:"base_url"`
	StartURL     string        `yaml:"start_url"`
	Delay        time.Duration `yaml:"delay"`
	Parallel     int           `yaml:"parallel"`
	BooksIdsFile string        `yaml:"books_ids_file"`
	OutputFile   string        `yaml:"output_file"`
	ParseImages  bool          `yaml:"parse_images"`
}

type LoggerConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func LoadConfig(filePath string) (Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Config{}, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func LoadUrls(filePath string) ([]string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	var bookIDs []string
	if err := json.Unmarshal(data, &bookIDs); err != nil {
		return nil, err
	}

	return bookIDs, nil
}
