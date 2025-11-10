package main

import (
	"fmt"

	"docker-log-parser/pkg/logs"
)

// This example demonstrates how ANSI escape codes are used as delineators
// to identify log entry boundaries in multi-line log streams.
func main() {
	// Sample log lines with ANSI color codes
	// Lines starting with ANSI codes (e.g., \x1b[32m) are typically new log entries
	// Lines with leading whitespace are continuation lines
	testLines := []string{
		"\x1b[32mOct  3 21:53:27.208471\x1b[0m \x1b[34mINF\x1b[0m Application started successfully",
		"  Loading configuration from /etc/app/config.yaml",
		"  Connecting to database at localhost:5432",
		"\x1b[32mOct  3 21:53:28.456789\x1b[0m \x1b[33mWARN\x1b[0m Database connection slow",
		"\x1b[32mOct  3 21:53:29.123456\x1b[0m \x1b[31mERROR\x1b[0m Failed to execute query",
		"  SQL: SELECT * FROM users",
		"       WHERE id = $1",
		"       AND status = 'active'",
		"  Error: connection timeout",
		"\x1b[32mOct  3 21:53:30.789012\x1b[0m \x1b[34mINF\x1b[0m Retrying connection",
	}

	fmt.Println("ANSI Escape Code Delineation Example")
	fmt.Println("=====================================\n")
	fmt.Println("This demonstrates how ANSI escape codes help identify log boundaries.")
	fmt.Println("The parser uses multiple heuristics including:")
	fmt.Println("  - Lines starting with ANSI codes (\\x1b[)")
	fmt.Println("  - Timestamps at the beginning")
	fmt.Println("  - Log levels (INFO, WARN, ERROR, etc.)")
	fmt.Println("  - Leading whitespace (indicates continuation)")
	fmt.Println()

	for i, line := range testLines {
		isNew := logs.IsLikelyNewLogEntry(line)
		marker := "   "
		if isNew {
			marker = ">>>"
		}

		// Show first 70 chars for display
		display := line
		if len(line) > 70 {
			display = line[:70] + "..."
		}
		fmt.Printf("%s Line %2d: IsNewEntry=%5v | %q\n", marker, i+1, isNew, display)
	}

	fmt.Println("\n>>> indicates a NEW log entry (detected by ANSI codes and other heuristics)")
	fmt.Println("    indicates a CONTINUATION line (leading whitespace, no ANSI at start)")
	fmt.Println("\nThis allows the log viewer to correctly group multi-line logs together,")
	fmt.Println("such as SQL queries, stack traces, and multi-line error messages.")
}
