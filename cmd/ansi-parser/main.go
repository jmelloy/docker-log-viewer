package main

import (
	"fmt"
	"regexp"
)

// Block represents a text segment with its associated ANSI codes
type Block struct {
	Text      string
	ANSICodes []string // The ANSI codes applied to this block
}

// ParseANSIBlocks splits a string into blocks based on ANSI escape codes
func ParseANSIBlocks(s string) []Block {
	// Regex to match ANSI escape codes like \x1b[90m, \x1b[0m, etc.
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)

	var blocks []Block
	var currentCodes []string
	lastIdx := 0

	// Find all ANSI code positions
	matches := ansiRegex.FindAllStringIndex(s, -1)

	for _, match := range matches {
		start, end := match[0], match[1]

		// If there's text before this ANSI code, create a block
		if start > lastIdx {
			text := s[lastIdx:start]
			blocks = append(blocks, Block{
				Text:      text,
				ANSICodes: append([]string{}, currentCodes...), // Copy current codes
			})
		}

		// Extract the ANSI code
		code := s[start:end]

		// Check if it's a reset code (\x1b[0m or \x1b[0m)
		if code == "\x1b[0m" {
			currentCodes = []string{}
		} else {
			currentCodes = append(currentCodes, code)
		}

		lastIdx = end
	}

	// Add remaining text after last ANSI code
	if lastIdx < len(s) {
		text := s[lastIdx:]
		blocks = append(blocks, Block{
			Text:      text,
			ANSICodes: append([]string{}, currentCodes...),
		})
	}

	return blocks
}

// Example usage
func main() {
	input := `\x1b[90mNov 10 07:44:01.481833\x1b[0m \x1b[34mTRC\x1b[0m \x1b[1mpkg/repository/app/repository.go:462\x1b[0m\x1b[36m >\x1b[0m [sql]: SELECT *, workspace_app_id AS loader_key FROM "workspace_app_webhooks" WHERE workspace_app_id IN ($1,$2) AND "workspace_app_webhooks"."deleted_at" IS NULL \x1b[36mdb.rows=\x1b[0m2 \x1b[36mdb.table=\x1b[0mworkspace_app_webhooks \x1b[36mdb.vars=\x1b[0m["3553Aj7JSO7I0xkTKa6GHa7fx0f","3553AjGLnIaAB9umMjUBijXiLPk"] \x1b[36mduration=\x1b[0m68.405061 \x1b[36mgql.operationName=\x1b[0mWorkspaceApps \x1b[36mlocation=\x1b[0m["/app/pkg/repository/app/repository.go:462","/app/pkg/repository/loader/loader.go:235","/app/pkg/repository/loader/loader.go:217"] \x1b[36mrequest_id=\x1b[0m135bd469 \x1b[36mtrace_id=\x1b[0m201f6da13a80b65b9219122f62ce7118`

	blocks := ParseANSIBlocks(input)

	for i, block := range blocks {
		fmt.Printf("Block %d:\n", i)
		fmt.Printf("  Text: %q\n", block.Text)
		fmt.Printf("  ANSI Codes: %v\n", block.ANSICodes)
		fmt.Println()
	}
}
