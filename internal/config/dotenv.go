package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotenv reads a .env file from projectPath and returns key-value pairs.
// Returns an empty map (no error) if the file does not exist.
// Supports: KEY=VALUE, # comments, blank lines, quoted values (single/double).
func LoadDotenv(projectPath string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, ".env"))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading .env: %w", err)
	}
	return parseDotenv(string(data))
}

func parseDotenv(content string) (map[string]string, error) {
	env := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Strip matching surrounding quotes.
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}
	return env, nil
}
