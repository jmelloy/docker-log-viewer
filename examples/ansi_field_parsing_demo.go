package main

import (
	"fmt"

	"docker-log-parser/pkg/logs"
)

// This example demonstrates how ANSI escape codes can be used as field boundary hints
// in the key=value field parser
func main() {
	// Sample log lines with ANSI codes in various positions
	testLines := []string{
		// ANSI codes around field names
		"\x1b[32mOct  3 21:53:27\x1b[0m INF Processing \x1b[33mrequest_id\x1b[0m=abc123 \x1b[36muser_id\x1b[0m=456",

		// ANSI codes in field values
		"Oct  3 21:53:27 INF Processing request_id=\x1b[33mabc123\x1b[0m status=\x1b[32msuccess\x1b[0m",

		// ANSI codes as field delimiters
		"Oct  3 21:53:27 TRC pkg/test.go:42 > Query \x1b[36mdb.table\x1b[0m=users \x1b[33mduration\x1b[0m=1.234 \x1b[35mrows\x1b[0m=42",

		// No ANSI codes (regular parsing)
		"Oct  3 21:53:27 INF Processing request_id=xyz789 user_id=123 status=complete",
	}

	fmt.Println("ANSI-Aware Field Boundary Parsing Example")
	fmt.Println("==========================================\n")
	fmt.Println("This demonstrates how ANSI escape codes can help identify field boundaries")
	fmt.Println("in key=value structured logs. ANSI codes around field names or between fields")
	fmt.Println("serve as additional hints for parsing, especially when spacing is ambiguous.\n")

	for i, line := range testLines {
		fmt.Printf("=== Example %d ===\n", i+1)

		// Show the line with visible ANSI codes
		displayLine := line
		if len(line) > 90 {
			displayLine = line[:90] + "..."
		}
		fmt.Printf("Raw: %q\n", displayLine)

		entry := logs.ParseLogLine(line)

		fmt.Printf("Timestamp: %s\n", entry.Timestamp)
		fmt.Printf("Level:     %s\n", entry.Level)
		fmt.Printf("Message:   %s\n", entry.Message)

		if len(entry.Fields) > 0 {
			fmt.Printf("Fields:\n")
			for k, v := range entry.Fields {
				fmt.Printf("  %s = %s\n", k, v)
			}
		}
		fmt.Println()
	}

	fmt.Println("Benefits:")
	fmt.Println("- ANSI codes around field names help identify key boundaries")
	fmt.Println("- ANSI codes in values are automatically stripped")
	fmt.Println("- Fallback to regular parsing when no ANSI codes present")
	fmt.Println("- Works seamlessly with colored log output from applications")
}
