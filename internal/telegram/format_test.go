package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bold with **",
			input:    "**bold**",
			expected: "<b>bold</b>",
		},
		{
			name:     "bold with __",
			input:    "__bold__",
			expected: "<b>bold</b>",
		},
		{
			name:     "italic with *",
			input:    "This is *italic* text",
			expected: "This is <i>italic</i> text",
		},
		{
			name:     "italic with _",
			input:    "This is _italic_ text",
			expected: "This is <i>italic</i> text",
		},
		{
			name:     "inline code",
			input:    "Use `const x = 10;` in code",
			expected: "Use <code>const x = 10;</code> in code",
		},
		{
			name:     "code block without language",
			input:    "```\nfunc main() {\n}\n```",
			expected: "<pre>func main() {\n}</pre>",
		},
		{
			name:     "code block with language",
			input:    "```go\nfunc main() {\n}\n```",
			expected: "<pre><code class=\"language-go\">func main() {\n}</code></pre>",
		},
		{
			name:     "strikethrough",
			input:    "~~deleted~~",
			expected: "<s>deleted</s>",
		},
		{
			name:     "link",
			input:    "[Google](https://google.com)",
			expected: "<a href=\"https://google.com\">Google</a>",
		},
		{
			name:     "blockquote",
			input:    "> This is a quote",
			expected: "<blockquote>This is a quote</blockquote>",
		},
		{
			name:     "heading",
			input:    "# Main Title",
			expected: "<b>Main Title</b>",
		},
		{
			name:     "list with -",
			input:    "- First item\n- Second item",
			expected: "• First item\n• Second item",
		},
		{
			name:     "list with *",
			input:    "* First item",
			expected: "• First item",
		},
		{
			name:     "HTML entities escaped",
			input:    "Use <script> tag",
			expected: "Use &lt;script&gt; tag",
		},
		{
			name:     "HTML entities in code preserved",
			input:    "`<div>test</div>`",
			expected: "<code>&lt;div&gt;test&lt;/div&gt;</code>",
		},
		{
			name:     "mixed formatting",
			input:    "**bold** and *italic* with `code`",
			expected: "<b>bold</b> and <i>italic</i> with <code>code</code>",
		},
		{
			name:     "complex markdown",
			input:    "# Title\n\n**Bold** text with *italic* and `code`.\n\n```go\nfunc() {}\n```\n\n> Quote",
			expected: "<b>Title</b>\n\n<b>Bold</b> text with <i>italic</i> and <code>code</code>.\n\n<pre><code class=\"language-go\">func() {}</code></pre>\n\n<blockquote>Quote</blockquote>",
		},
		{
			name:     "ampersand escaping",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "ampersand in code",
			input:    "`a && b`",
			expected: "<code>a &amp;&amp; b</code>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatHTML(tt.input)
			if result != tt.expected {
				t.Errorf("FormatHTML(%q)\n  got:  %q\n  want: %q", tt.input, result, tt.expected)
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

func TestSplitMessageWithHTMLTags(t *testing.T) {
	// Create a long message with HTML tags that will need splitting
	longText := "<b>Bold text " + strings.Repeat("word ", 1000) + "</b>"

	chunks := SplitMessage(longText, 4096)

	if len(chunks) < 2 {
		t.Skip("Text not long enough to split")
	}

	// Verify all chunks are within limit
	for i, chunk := range chunks {
		if len(chunk) > 4096 {
			t.Errorf("Chunk %d exceeds limit: %d", i, len(chunk))
		}
	}

	// Verify tags are balanced
	for i, chunk := range chunks {
		openCount := strings.Count(chunk, "<b>")
		closeCount := strings.Count(chunk, "</b>")
		if openCount != closeCount {
			t.Errorf("Chunk %d has unbalanced <b> tags: %d open, %d close", i, openCount, closeCount)
		}
	}

	// Recombine by removing duplicate tags at boundaries
	recombined := chunks[0]
	for i := 1; i < len(chunks); i++ {
		// Remove closing tags from previous chunk and opening tags from current chunk
		recombined = strings.TrimSuffix(recombined, "</b>")
		current := strings.TrimPrefix(chunks[i], "<b>")
		recombined += current
	}

	if recombined != longText {
		t.Error("Recombined chunks don't match original text")
	}
}

func TestSplitMessageNoSplitInsideHTMLTag(t *testing.T) {
	// Test that we don't split in the middle of an HTML tag
	text := strings.Repeat("x", 4090) + "<a href=\"https://example.com\">link</a>"
	chunks := SplitMessage(text, 4096)

	// Verify we don't have broken tags
	for i, chunk := range chunks {
		// Count < and > to ensure they're balanced
		openBrackets := strings.Count(chunk, "<")
		closeBrackets := strings.Count(chunk, ">")

		// Each opening bracket should have a matching closing bracket
		// (this is a simple check, not perfect but catches most issues)
		if openBrackets != closeBrackets {
			t.Errorf("Chunk %d has unbalanced brackets: %d < and %d >", i, openBrackets, closeBrackets)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
