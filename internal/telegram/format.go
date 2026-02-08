package telegram

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// FormatHTML converts markdown-style text to Telegram-compatible HTML
// Supports: bold, italic, code, code blocks, strikethrough, links, blockquotes, headings, lists
func FormatHTML(text string) string {
	// First, escape HTML entities in the entire text
	// We'll temporarily protect markdown patterns during escaping
	result := text

	// Step 1: Extract and protect code blocks (``` fenced blocks)
	codeBlockRegex := regexp.MustCompile("```([a-z]*)\n([\\s\\S]*?)\n?```")
	protectedCodeBlocks := make(map[string]string)
	placeholder := 0

	result = codeBlockRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := codeBlockRegex.FindStringSubmatch(match)
		lang := matches[1]
		code := matches[2]

		// Escape HTML in code content
		escapedCode := html.EscapeString(code)

		var htmlCode string
		if lang != "" {
			htmlCode = "<pre><code class=\"language-" + lang + "\">" + escapedCode + "</code></pre>"
		} else {
			htmlCode = "<pre>" + escapedCode + "</pre>"
		}

		key := "\x00CODEBLOCK" + string(rune(placeholder)) + "\x00"
		protectedCodeBlocks[key] = htmlCode
		placeholder++
		return key
	})

	// Step 2: Extract and protect inline code (` backticks)
	inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
	protectedInlineCode := make(map[string]string)

	result = inlineCodeRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := inlineCodeRegex.FindStringSubmatch(match)
		code := matches[1]

		// Escape HTML in code content
		escapedCode := html.EscapeString(code)
		htmlCode := "<code>" + escapedCode + "</code>"

		key := "\x00INLINECODE" + string(rune(placeholder)) + "\x00"
		protectedInlineCode[key] = htmlCode
		placeholder++
		return key
	})

	// Step 3: Extract and protect markdown tables
	tableRegex := regexp.MustCompile(`(?m)^(\|.+\|)\n(\|[\s\-:]+\|)\n((?:\|.+\|\n?)+)`)
	protectedTables := make(map[string]string)

	result = tableRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Convert table to preformatted text (Telegram doesn't support tables)
		// Escape HTML in table content
		escapedTable := html.EscapeString(match)
		htmlTable := "<pre>" + escapedTable + "</pre>"

		key := "\x00TABLE" + string(rune(placeholder)) + "\x00"
		protectedTables[key] = htmlTable
		placeholder++
		return key
	})

	// Step 4: Escape remaining HTML entities in non-code text
	result = html.EscapeString(result)

	// Step 5: Convert markdown patterns to HTML (in order of precedence)

	// Bold: **text** or __text__
	boldRegex := regexp.MustCompile(`\*\*([^\*]+)\*\*|__([^_]+)__`)
	result = boldRegex.ReplaceAllStringFunc(result, func(match string) string {
		content := strings.TrimPrefix(strings.TrimSuffix(match, "**"), "**")
		content = strings.TrimPrefix(strings.TrimSuffix(content, "__"), "__")
		return "<b>" + content + "</b>"
	})

	// Italic: *text* or _text_ (but not inside words)
	italicRegex := regexp.MustCompile(`(?:^|[^\w*])\*([^\*]+)\*(?:[^\w*]|$)|(?:^|[^\w_])_([^_]+)_(?:[^\w_]|$)`)
	result = italicRegex.ReplaceAllStringFunc(result, func(match string) string {
		// Preserve leading/trailing characters
		var prefix, suffix string
		content := match

		if len(match) > 0 && !isMarkdownChar(match[0]) {
			prefix = string(match[0])
			content = content[1:]
		}
		if len(content) > 0 && !isMarkdownChar(content[len(content)-1]) {
			suffix = string(content[len(content)-1])
			content = content[:len(content)-1]
		}

		content = strings.TrimPrefix(strings.TrimSuffix(content, "*"), "*")
		content = strings.TrimPrefix(strings.TrimSuffix(content, "_"), "_")
		return prefix + "<i>" + content + "</i>" + suffix
	})

	// Strikethrough: ~~text~~
	strikeRegex := regexp.MustCompile(`~~([^~]+)~~`)
	result = strikeRegex.ReplaceAllString(result, "<s>$1</s>")

	// Links: [text](url)
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	result = linkRegex.ReplaceAllString(result, `<a href="$2">$1</a>`)

	// Blockquotes: > text (at start of line)
	blockquoteRegex := regexp.MustCompile(`(?m)^&gt;\s*(.+)$`)
	result = blockquoteRegex.ReplaceAllString(result, "<blockquote>$1</blockquote>")

	// Headings: # Heading (convert to bold)
	headingRegex := regexp.MustCompile(`(?m)^#+\s*(.+)$`)
	result = headingRegex.ReplaceAllString(result, "<b>$1</b>")

	// Lists: - item or * item (convert to bullet)
	listRegex := regexp.MustCompile(`(?m)^[\-\*]\s+(.+)$`)
	result = listRegex.ReplaceAllString(result, "• $1")

	// Step 6: Restore protected sections (order matters: tables first, then code)
	for key, value := range protectedTables {
		result = strings.ReplaceAll(result, key, value)
	}
	for key, value := range protectedInlineCode {
		result = strings.ReplaceAll(result, key, value)
	}
	for key, value := range protectedCodeBlocks {
		result = strings.ReplaceAll(result, key, value)
	}

	return result
}

// isMarkdownChar checks if a byte is a markdown formatting character
func isMarkdownChar(b byte) bool {
	return b == '*' || b == '_'
}

// SplitMessage splits a message into chunks that fit within the limit
// Splits at paragraph boundaries (\n\n), then line boundaries (\n), then word boundaries
// Handles HTML tag splitting by closing and reopening them
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

		// Account for tag balancing overhead when finding split position
		// Reserve space for closing tags
		maxTagOverhead := 100 // Maximum bytes for closing/opening tags
		adjustedLimit := limit - maxTagOverhead
		if adjustedLimit < limit/2 {
			adjustedLimit = limit / 2
		}

		splitPos := findSplitPosition(remaining, adjustedLimit)

		chunk := remaining[:splitPos]
		remaining = remaining[splitPos:]

		// Check if we're splitting inside an HTML tag
		chunkStartInOriginal := len(text) - len(remaining) - len(chunk)
		if isInsideHTMLTag(text, chunkStartInOriginal+len(chunk)) {
			// Find the end of the current tag
			tagEnd := strings.Index(remaining, ">")
			if tagEnd != -1 && tagEnd < 100 { // Reasonable tag length
				// Include the rest of the tag in this chunk
				chunk += remaining[:tagEnd+1]
				remaining = remaining[tagEnd+1:]
			}
		}

		// Balance HTML tags at split point
		chunk, remaining = balanceHTMLTags(chunk, remaining)

		// If balancing pushed us over limit, try splitting earlier
		if len(chunk) > limit {
			// Find open tags to calculate actual overhead
			openTags := findOpenTags(chunk)
			tagOverhead := 0
			for _, tag := range openTags {
				tagOverhead += len(tag) + 3 // </tag>
			}

			// Retry with adjusted limit
			newLimit := adjustedLimit - tagOverhead
			if newLimit < limit/3 {
				newLimit = limit / 3
			}

			splitPos = findSplitPosition(text, newLimit)
			chunk = remaining[:splitPos]
			remaining = remaining[splitPos:]
			chunk, remaining = balanceHTMLTags(chunk, remaining)
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

// isInsideHTMLTag checks if the position is inside an HTML tag
func isInsideHTMLTag(text string, pos int) bool {
	if pos >= len(text) {
		return false
	}

	// Look backwards for the last < or >
	beforeText := text[:pos]
	lastOpen := strings.LastIndex(beforeText, "<")
	lastClose := strings.LastIndex(beforeText, ">")

	// If the last < comes after the last >, we're inside a tag
	return lastOpen > lastClose
}

// balanceHTMLTags ensures HTML tags are balanced at split points
// Closes open tags at end of chunk and reopens them at start of next chunk
func balanceHTMLTags(chunk, remaining string) (string, string) {
	// Find all open tags in the chunk
	openTags := findOpenTags(chunk)

	if len(openTags) == 0 {
		return chunk, remaining
	}

	// Close tags at end of chunk (in reverse order)
	for i := len(openTags) - 1; i >= 0; i-- {
		chunk += "</" + openTags[i] + ">"
	}

	// Reopen tags at start of next chunk
	for _, tag := range openTags {
		remaining = "<" + tag + ">" + remaining
	}

	return chunk, remaining
}

// findOpenTags finds all unclosed HTML tags in the text
func findOpenTags(text string) []string {
	var openTags []string
	tagStack := make([]string, 0)

	// Simple HTML tag parser
	tagRegex := regexp.MustCompile(`<(/?)([a-z]+)(?:\s[^>]*)?>`)
	matches := tagRegex.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		isClosing := match[1] == "/"
		tagName := match[2]

		// Skip self-closing tags
		if tagName == "br" {
			continue
		}

		if isClosing {
			// Pop from stack if it matches
			if len(tagStack) > 0 && tagStack[len(tagStack)-1] == tagName {
				tagStack = tagStack[:len(tagStack)-1]
			}
		} else {
			// Push to stack
			tagStack = append(tagStack, tagName)
		}
	}

	// Return remaining open tags
	openTags = tagStack
	return openTags
}
