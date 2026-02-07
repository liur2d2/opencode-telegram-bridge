package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOffsetFileNotFound(t *testing.T) {
	// Test first run - file doesn't exist
	tempDir := t.TempDir()
	offsetFile := filepath.Join(tempDir, "nonexistent_offset")

	offset, err := LoadOffset(offsetFile)
	if err != nil {
		t.Fatalf("LoadOffset should return 0 for missing file, got error: %v", err)
	}
	if offset != 0 {
		t.Fatalf("LoadOffset should return 0 for first run, got: %d", offset)
	}
}

func TestLoadOffsetValidFile(t *testing.T) {
	// Test loading valid offset from file
	tempDir := t.TempDir()
	offsetFile := filepath.Join(tempDir, "offset")

	// Save a known offset
	testOffset := int64(12345)
	if err := SaveOffset(offsetFile, testOffset); err != nil {
		t.Fatalf("SaveOffset failed: %v", err)
	}

	// Load it back
	offset, err := LoadOffset(offsetFile)
	if err != nil {
		t.Fatalf("LoadOffset failed: %v", err)
	}
	if offset != testOffset {
		t.Fatalf("LoadOffset returned %d, expected %d", offset, testOffset)
	}
}

func TestLoadOffsetEmptyFile(t *testing.T) {
	// Test loading empty file
	tempDir := t.TempDir()
	offsetFile := filepath.Join(tempDir, "offset")

	// Create empty file
	if err := os.WriteFile(offsetFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	offset, err := LoadOffset(offsetFile)
	if err != nil {
		t.Fatalf("LoadOffset should handle empty file, got error: %v", err)
	}
	if offset != 0 {
		t.Fatalf("LoadOffset should return 0 for empty file, got: %d", offset)
	}
}

func TestLoadOffsetInvalidContent(t *testing.T) {
	// Test loading file with invalid content
	tempDir := t.TempDir()
	offsetFile := filepath.Join(tempDir, "offset")

	// Create file with invalid content
	if err := os.WriteFile(offsetFile, []byte("not_a_number"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := LoadOffset(offsetFile)
	if err == nil {
		t.Fatal("LoadOffset should return error for invalid content")
	}
}

func TestSaveOffsetAtomicity(t *testing.T) {
	// Test atomic write behavior
	tempDir := t.TempDir()
	offsetFile := filepath.Join(tempDir, "offset")

	// Write first offset
	offset1 := int64(100)
	if err := SaveOffset(offsetFile, offset1); err != nil {
		t.Fatalf("SaveOffset failed: %v", err)
	}

	// Verify file exists and is correct
	loaded, err := LoadOffset(offsetFile)
	if err != nil {
		t.Fatalf("LoadOffset failed: %v", err)
	}
	if loaded != offset1 {
		t.Fatalf("First write: expected %d, got %d", offset1, loaded)
	}

	// Write second offset to same file
	offset2 := int64(200)
	if err := SaveOffset(offsetFile, offset2); err != nil {
		t.Fatalf("SaveOffset failed: %v", err)
	}

	// Verify second write
	loaded, err = LoadOffset(offsetFile)
	if err != nil {
		t.Fatalf("LoadOffset failed: %v", err)
	}
	if loaded != offset2 {
		t.Fatalf("Second write: expected %d, got %d", offset2, loaded)
	}

	// Verify temp file is cleaned up
	tempFile := offsetFile + ".tmp"
	if _, err := os.Stat(tempFile); err == nil {
		t.Fatal("Temp file should be cleaned up after successful write")
	}
}

func TestSaveOffsetNoTempFileLeftOnFailure(t *testing.T) {
	// Test that temp file is cleaned up on rename failure
	tempDir := t.TempDir()
	offsetFile := filepath.Join(tempDir, "offset")

	// Create a directory instead of file path to cause rename failure
	os.Mkdir(offsetFile, 0755)

	// This should fail to rename
	err := SaveOffset(offsetFile, 12345)
	if err == nil {
		t.Fatal("SaveOffset should fail when target is a directory")
	}

	// Verify temp file is cleaned up
	tempFile := offsetFile + ".tmp"
	if _, err := os.Stat(tempFile); err == nil {
		t.Fatal("Temp file should be cleaned up after failed rename")
	}
}

func TestSaveOffsetWithTildeExpansion(t *testing.T) {
	// Test tilde expansion in path
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	// Use temp dir but test with absolute path
	tempDir := t.TempDir()
	absolutePath := filepath.Join(home, "test-offset-"+filepath.Base(tempDir))
	defer os.Remove(absolutePath)

	// Save with absolute path
	offset := int64(999)
	if err := SaveOffset(absolutePath, offset); err != nil {
		t.Fatalf("SaveOffset failed: %v", err)
	}

	// Verify
	loaded, err := LoadOffset(absolutePath)
	if err != nil {
		t.Fatalf("LoadOffset failed: %v", err)
	}
	if loaded != offset {
		t.Fatalf("Expected %d, got %d", offset, loaded)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			input:    "relative/path",
			expected: "relative/path",
		},
		{
			input:    "~/.config/myapp",
			expected: filepath.Join(home, ".config/myapp"),
		},
		{
			input:    "~",
			expected: home,
		},
	}

	for _, test := range tests {
		got, err := expandHome(test.input)
		if err != nil {
			t.Fatalf("expandHome(%q) failed: %v", test.input, err)
		}
		if got != test.expected {
			t.Fatalf("expandHome(%q): expected %q, got %q", test.input, test.expected, got)
		}
	}
}
