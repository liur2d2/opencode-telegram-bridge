package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMarkdownV2Escape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape special characters",
			input:    "Hello_world*test",
			expected: "Hello\\_world\\*test",
		},
		{
			name:     "escape brackets",
			input:    "[link](url)",
			expected: "\\[link\\]\\(url\\)",
		},
		{
			name:     "escape various symbols",
			input:    "Price: $100 - #1 item!",
			expected: "Price: $100 \\- \\#1 item\\!",
		},
		{
			name:     "escape dots and pipes",
			input:    "v1.0 | status",
			expected: "v1\\.0 \\| status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdownV2(tt.input)
			if result != tt.expected {
				t.Errorf("FormatMarkdownV2(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCodeBlockPreserve(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "preserve code block",
			input:    "Check this code:\n```\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```\nNice, right?",
			expected: "Check this code:\n```\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```\nNice, right?",
		},
		{
			name:     "escape outside code block",
			input:    "Text_with_underscores\n```\ncode_with_underscores\n```\nMore_text",
			expected: "Text\\_with\\_underscores\n```\ncode_with_underscores\n```\nMore\\_text",
		},
		{
			name:     "preserve inline code",
			input:    "Use `const value = 10;` in your code",
			expected: "Use `const value = 10;` in your code",
		},
		{
			name:     "preserve bold",
			input:    "This is *bold* text",
			expected: "This is *bold* text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMarkdownV2(tt.input)
			if result != tt.expected {
				t.Errorf("FormatMarkdownV2(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		limit    int
		expected int // expected number of chunks
	}{
		{
			name:     "short message no split",
			input:    "Hello, World!",
			limit:    4096,
			expected: 1,
		},
		{
			name:     "exact limit no split",
			input:    strings.Repeat("a", 4096),
			limit:    4096,
			expected: 1,
		},
		{
			name:     "split at paragraph boundary",
			input:    strings.Repeat("paragraph\n\n", 500),
			limit:    4096,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitMessage(tt.input, tt.limit)
			if len(result) != tt.expected {
				t.Errorf("SplitMessage produced %d chunks, expected %d", len(result), tt.expected)
			}

			// Verify all chunks are within limit
			for i, chunk := range result {
				if len(chunk) > tt.limit {
					t.Errorf("Chunk %d exceeds limit: %d > %d", i, len(chunk), tt.limit)
				}
			}
		})
	}
}

func TestSplitMessageExactLimit(t *testing.T) {
	text := strings.Repeat("x", 4096)
	chunks := SplitMessage(text, 4096)

	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for exactly 4096 chars, got %d", len(chunks))
	}

	if chunks[0] != text {
		t.Error("Chunk doesn't match original text")
	}
}

func TestSplitMessageUnicode(t *testing.T) {
	// Test with multi-byte unicode characters
	text := strings.Repeat("你好世界", 2000) // Chinese characters
	chunks := SplitMessage(text, 4096)

	// Verify all chunks are valid UTF-8 and within limit
	for i, chunk := range chunks {
		if len(chunk) > 4096 {
			t.Errorf("Chunk %d exceeds limit: %d", i, len(chunk))
		}
		if !utf8.ValidString(chunk) {
			t.Errorf("Chunk %d contains invalid UTF-8", i)
		}
	}

	// Recombine and verify we didn't lose data
	recombined := strings.Join(chunks, "")
	if recombined != text {
		t.Error("Recombined chunks don't match original text")
	}
}

func TestSplitMessageMidCodeBlock(t *testing.T) {
	// Create a long code block that will need splitting
	longCode := "```\n"
	for i := 0; i < 100; i++ {
		longCode += "func example" + strings.Repeat("X", 40) + "() {\n"
		longCode += "    // This is a comment\n"
		longCode += "}\n"
	}
	longCode += "```\n"

	chunks := SplitMessage(longCode, 4096)

	if len(chunks) < 2 {
		t.Skip("Code block not long enough to split")
	}

	// Each chunk except the last should end with ```
	for i := 0; i < len(chunks)-1; i++ {
		if !strings.HasSuffix(chunks[i], "```\n") && !strings.HasSuffix(chunks[i], "```") {
			t.Errorf("Chunk %d doesn't end with code block close: %q", i, chunks[i][len(chunks[i])-10:])
		}
	}

	// Each chunk except the first should start with ```
	for i := 1; i < len(chunks); i++ {
		if !strings.HasPrefix(chunks[i], "```\n") && !strings.HasPrefix(chunks[i], "```") {
			t.Errorf("Chunk %d doesn't start with code block open: %q", i, chunks[i][:min(10, len(chunks[i]))])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
