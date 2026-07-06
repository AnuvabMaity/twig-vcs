package objects

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ReadConfig reads a minimal key=value config file at path.
// It ignores empty lines, leading/trailing whitespace, and comments starting with '#'.
func ReadConfig(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: invalid key=value format (missing '=')", lineNum)
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNum)
		}

		config[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning config file: %w", err)
	}

	return config, nil
}

// WriteConfig writes key-value pairs formatted as key=value lines to the file at path.
// It ensures that any missing parent directories are created first.
func WriteConfig(path string, values map[string]string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for k, v := range values {
		if _, err := writer.WriteString(fmt.Sprintf("%s=%s\n", k, v)); err != nil {
			return fmt.Errorf("writing to config: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flushing config writer: %w", err)
	}

	return nil
}

// ResolveAuthorID reads user.id from the config file if present;
// otherwise it falls back to the current OS username.
// Returns an error if neither can be resolved.
func ResolveAuthorID(path string) (string, error) {
	config, err := ReadConfig(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) && !os.IsNotExist(err) {
			return "", fmt.Errorf("reading config file: %w", err)
		}
	}

	if config != nil {
		if authorID, ok := config["user.id"]; ok && authorID != "" {
			return authorID, nil
		}
	}

	// Fallback to current OS username
	currUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolving OS username: %w", err)
	}

	// In some environments, Username can be empty or have domain prefix.
	// But it shouldn't be empty.
	if currUser.Username == "" {
		return "", errors.New("current OS user has empty username")
	}

	return currUser.Username, nil
}
