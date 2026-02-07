package telegram

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// MarkdownV2 special characters that need escaping outside code blocks
var markdownV2SpecialChars = []string{
	"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!",
}

// FormatMarkdownV2 formats text for Telegram's MarkdownV2 parse mode
// Preserves code blocks (``` and `), bold (*text*), and escapes special characters elsewhere
func FormatMarkdownV2(text string) string {
	// Don't escape anything for now - we'll handle this in the actual implementation
	// For the initial tests, we need to escape special chars outside of code blocks and bold

	// First, identify and protect code blocks and inline code
	// We'll use placeholders to protect them during escaping

	result := text

	// Find all code blocks (```) and inline code (`)
	codeBlockRegex := regexp.MustCompile("```[\\s\\S]*?```")
	inlineCodeRegex := regexp.MustCompile("`[^`]+`")
	boldRegex := regexp.MustCompile("\\*[^*]+\\*")

	// Store protected sections
	protectedSections := make(map[string]string)
	placeholder := 0

	// Protect code blocks
	result = codeBlockRegex.ReplaceAllStringFunc(result, func(match string) string {
		key := "\x00CODEBLOCK" + string(rune(placeholder)) + "\x00"
		protectedSections[key] = match
		placeholder++
		return key
	})

	// Protect inline code
	result = inlineCodeRegex.ReplaceAllStringFunc(result, func(match string) string {
		key := "\x00INLINECODE" + string(rune(placeholder)) + "\x00"
		protectedSections[key] = match
		placeholder++
		return key
	})

	// Protect bold
	result = boldRegex.ReplaceAllStringFunc(result, func(match string) string {
		key := "\x00BOLD" + string(rune(placeholder)) + "\x00"
		protectedSections[key] = match
		placeholder++
		return key
	})

	// Escape special characters in the remaining text
	for _, char := range markdownV2SpecialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}

	// Restore protected sections
	for key, value := range protectedSections {
		result = strings.ReplaceAll(result, key, value)
	}

	return result
}

// SplitMessage splits a message into chunks that fit within the limit
// Splits at paragraph boundaries (\n\n), then line boundaries (\n), then word boundaries
// Handles code block splitting by closing and reopening them
func SplitMessage(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= limit {
			chunks = append(chunks, remaining)
			break
		}

		splitPos := findSplitPosition(remaining, limit)

		chunk := remaining[:splitPos]
		remaining = remaining[splitPos:]

		chunkStartInOriginal := len(text) - len(remaining) - len(chunk)
		if isInsideCodeBlock(text, chunkStartInOriginal+len(chunk)) {
			if !strings.HasSuffix(chunk, "```") {
				chunk = strings.TrimRight(chunk, "\n") + "\n```\n"
			}
			if !strings.HasPrefix(remaining, "```") {
				remaining = "```\n" + remaining
			}
		}

		chunks = append(chunks, chunk)
	}

	return chunks
}

// findSplitPosition finds the best position to split the text within the limit
// Prefers paragraph boundaries, then line boundaries, then word boundaries
func findSplitPosition(text string, limit int) int {
	if len(text) <= limit {
		return len(text)
	}

	// Ensure we don't split in the middle of a multi-byte character
	// Walk backwards from limit to find a safe split point
	splitPos := limit
	for splitPos > 0 && !utf8.RuneStart(text[splitPos]) {
		splitPos--
	}

	// Try to find paragraph boundary (\n\n)
	if pos := strings.LastIndex(text[:splitPos], "\n\n"); pos > 0 && pos > splitPos/2 {
		return pos + 2 // Include the \n\n
	}

	// Try to find line boundary (\n)
	if pos := strings.LastIndex(text[:splitPos], "\n"); pos > 0 && pos > splitPos/2 {
		return pos + 1 // Include the \n
	}

	// Try to find word boundary (space)
	if pos := strings.LastIndex(text[:splitPos], " "); pos > 0 && pos > splitPos/2 {
		return pos + 1 // Include the space
	}

	// Fall back to character boundary
	return splitPos
}

// isInsideCodeBlock checks if the position is inside a code block
func isInsideCodeBlock(text string, pos int) bool {
	// Count code block markers before this position
	beforeText := text[:pos]
	count := strings.Count(beforeText, "```")
	// If odd number of markers, we're inside a code block
	return count%2 == 1
}
