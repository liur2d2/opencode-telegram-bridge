package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LoadOffset loads the saved update offset from a file.
// Returns 0 if the file doesn't exist (first run).
// Returns the highest update_id + 1 to avoid replaying old messages.
func LoadOffset(filePath string) (int64, error) {
	// Expand ~ to home directory
	expanded, err := expandHome(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to expand path: %w", err)
	}

	// Read the file
	data, err := os.ReadFile(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - first run
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read offset file: %w", err)
	}

	// Parse the offset as int64
	offsetStr := strings.TrimSpace(string(data))
	if offsetStr == "" {
		return 0, nil
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse offset: %w", err)
	}

	return offset, nil
}

// SaveOffset saves the update offset to a file atomically.
// Uses write-to-temp-file + rename pattern to prevent corruption.
func SaveOffset(filePath string, offset int64) error {
	// Expand ~ to home directory
	expanded, err := expandHome(filePath)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first
	tempFile := expanded + ".tmp"
	offsetBytes := []byte(strconv.FormatInt(offset, 10))

	if err := os.WriteFile(tempFile, offsetBytes, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomically rename temp file to target file
	if err := os.Rename(tempFile, expanded); err != nil {
		// Clean up temp file if rename failed
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// expandHome expands ~ to the user's home directory
func expandHome(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if path == "~" {
		return home, nil
	}

	return filepath.Join(home, path[2:]), nil
}
