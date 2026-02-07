package telegram

import (
	"encoding/base64"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestGetLargestPhoto(t *testing.T) {
	tests := []struct {
		name     string
		photos   []models.PhotoSize
		expected *models.PhotoSize
	}{
		{
			name:     "empty array",
			photos:   []models.PhotoSize{},
			expected: nil,
		},
		{
			name: "single photo",
			photos: []models.PhotoSize{
				{FileID: "photo1", Width: 100, Height: 100},
			},
			expected: &models.PhotoSize{FileID: "photo1", Width: 100, Height: 100},
		},
		{
			name: "multiple photos - pick largest by area",
			photos: []models.PhotoSize{
				{FileID: "photo1", Width: 100, Height: 100},
				{FileID: "photo2", Width: 200, Height: 200},
				{FileID: "photo3", Width: 150, Height: 150},
			},
			expected: &models.PhotoSize{FileID: "photo2", Width: 200, Height: 200},
		},
		{
			name: "multiple photos - different aspect ratios",
			photos: []models.PhotoSize{
				{FileID: "photo1", Width: 100, Height: 200},
				{FileID: "photo2", Width: 300, Height: 100},
				{FileID: "photo3", Width: 150, Height: 150},
			},
			expected: &models.PhotoSize{FileID: "photo2", Width: 300, Height: 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLargestPhoto(tt.photos)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}
			if result == nil {
				t.Fatalf("expected %+v, got nil", tt.expected)
			}
			if result.FileID != tt.expected.FileID || result.Width != tt.expected.Width || result.Height != tt.expected.Height {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestEncodeBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty data",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "simple data",
			input:    []byte("hello"),
			expected: "aGVsbG8=",
		},
		{
			name:     "binary data",
			input:    []byte{0xFF, 0xD8, 0xFF, 0xE0},
			expected: "/9j/4A==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeBase64(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}

			decoded, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("failed to decode base64: %v", err)
			}
			if string(decoded) != string(tt.input) {
				t.Errorf("decoded data doesn't match input")
			}
		})
	}
}
