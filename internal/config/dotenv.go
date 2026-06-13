package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDotEnv sets environment variables from a .env file when not already set.
// It searches the given path, then walks up parent directories.
func LoadDotEnv(path string) {
	for dir := searchRoot(); ; {
		candidate := filepath.Join(dir, path)
		if loadDotEnvFile(candidate) {
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func searchRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func loadDotEnvFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
	return true
}
