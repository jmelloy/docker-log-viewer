package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"docker-log-parser/pkg/logs"
)

func main() {
	var (
		containerID = flag.String("container", "", "Docker container ID or name to read logs from")
		logFile     = flag.String("file", "", "Log file path to read from")
		debug       = flag.Bool("debug", false, "Enable debug output")
		verbose     = flag.Bool("verbose", false, "Enable verbose output")
		skip        = flag.Int("skip", 0, "Number of recent log lines to skip (for Docker)")
		follow      = flag.Bool("follow", false, "Follow logs in real-time (Docker only)")
		filterLevel = flag.String("level", "", "Filter by log level (DBG, TRC, INF, WRN, ERR, FATAL)")
		filterText  = flag.String("search", "", "Filter by text (searches in message and fields)")
		csvExport   = flag.String("csv", "", "Export to CSV file (path to output file)")
	)
	flag.Parse()

	// Set up logging
	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if *containerID == "" && *logFile == "" {
		fmt.Println("Usage:")
		fmt.Println("  Read from Docker container:")
		fmt.Println("    test-parser -container <container_id_or_name> [-tail 100] [-follow]")
		fmt.Println("  Read from log file:")
		fmt.Println("    test-parser -file <log_file_path>")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -debug      Enable debug output")
		fmt.Println("  -verbose    Enable verbose output")
		fmt.Println("  -skip       Number of recent log lines to skip (for Docker)")
		fmt.Println("  -follow     Follow logs in real-time (Docker only)")
		fmt.Println("  -level      Filter by log level (DBG, TRC, INF, WRN, ERR, FATAL)")
		fmt.Println("  -search     Filter by text (searches in message and fields)")
		fmt.Println("  -csv        Export to CSV file (path to output file)")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  test-parser -file /var/log/app.log -level ERR")
		fmt.Println("  test-parser -file /var/log/app.log -search \"timeout\"")
		fmt.Println("  test-parser -file /var/log/app.log -csv output.csv")
		fmt.Println("  test-parser -file /var/log/app.log -level INF -csv output.csv")
		os.Exit(1)
	}

	if *containerID != "" && *logFile != "" {
		fmt.Println("Error: Cannot specify both container and file. Choose one.")
		os.Exit(1)
	}

	// Prepare CSV writer if export is requested
	var csvWriter *csv.Writer
	var csvFile *os.File
	var csvEntries []*logs.LogEntry
	var csvLineNumbers []int
	if *csvExport != "" {
		// When CSV export is requested, we collect entries first
		csvEntries = make([]*logs.LogEntry, 0)
		csvLineNumbers = make([]int, 0)
	}

	if *containerID != "" {
		readFromDockerContainer(*containerID, *skip, *follow, *debug, *verbose, *filterLevel, *filterText, &csvEntries, &csvLineNumbers)
	} else {
		readFromLogFile(*logFile, *debug, *verbose, *filterLevel, *filterText, &csvEntries, &csvLineNumbers)
	}

	// Write CSV file if requested
	if *csvExport != "" && len(csvEntries) > 0 {
		var err error
		csvFile, err = os.Create(*csvExport)
		if err != nil {
			fmt.Printf("Error creating CSV file: %v\n", err)
			os.Exit(1)
		}
		defer csvFile.Close()
		csvWriter = csv.NewWriter(csvFile)
		defer csvWriter.Flush()

		writeCSVFile(csvWriter, csvEntries, csvLineNumbers)
	}
}

func readFromDockerContainer(containerID string, skip int, follow bool, debug bool, verbose bool, filterLevel string, filterText string, csvEntries *[]*logs.LogEntry, csvLineNumbers *[]int) {
	fmt.Printf("Reading logs from Docker container: %s\n", containerID)
	if follow {
		fmt.Println("Following logs in real-time...")
	}
	fmt.Println(strings.Repeat("=", 80))

	ctx := context.Background()
	dockerClient, err := logs.NewDockerClient()
	if err != nil {
		fmt.Printf("Error creating Docker client: %v\n", err)
		os.Exit(1)
	}
	defer dockerClient.Close()

	linesSkipped := 0

	// List containers to help with debugging

	containers, err := dockerClient.ListRunningContainers(ctx)
	if err != nil {
		fmt.Printf("Error listing containers: %v\n", err)
	} else {
		fmt.Printf("Available containers:\n")
		for _, c := range containers {
			fmt.Printf("  %s (%s) - %s\n", c.ID, c.Name, c.Image)
		}
		fmt.Println()
	}

	// Create log channel
	logChan := make(chan logs.ContainerMessage, 1000)

	// Start streaming logs
	err = dockerClient.StreamLogs(ctx, containerID, logChan, nil)
	if err != nil {
		fmt.Printf("Error streaming logs: %v\n", err)
		os.Exit(1)
	}

	// Process logs
	lineCount := 0
	for {
		select {
		case logMsg := <-logChan:
			lineCount++
			if linesSkipped < skip {
				linesSkipped++
				continue
			}
			if shouldShowEntry(logMsg.Entry, filterLevel, filterText) {
				printLogEntry(lineCount, logMsg.Entry, debug, verbose)
				if csvEntries != nil {
					*csvEntries = append(*csvEntries, logMsg.Entry)
					*csvLineNumbers = append(*csvLineNumbers, lineCount)
				}
			}

		case <-time.After(5 * time.Second):
			if !follow {
				fmt.Printf("\nNo more logs available after 5 seconds.\n")
				return
			}
		}
	}
}

func readFromLogFile(filePath string, debug bool, verbose bool, filterLevel string, filterText string, csvEntries *[]*logs.LogEntry, csvLineNumbers *[]int) {
	fmt.Printf("Reading logs from file: %s\n", filePath)
	fmt.Println(strings.Repeat("=", 80))

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	buf := make([]byte, 8192)
	var leftover []byte
	var bufferedEntry *logs.LogEntry
	lineCount := 0
	entryCount := 0

	flushBuffered := func() {
		if bufferedEntry != nil {
			entryCount++
			if shouldShowEntry(bufferedEntry, filterLevel, filterText) {
				printLogEntry(entryCount, bufferedEntry, debug, verbose)
				if csvEntries != nil {
					*csvEntries = append(*csvEntries, bufferedEntry)
					*csvLineNumbers = append(*csvLineNumbers, entryCount)
				}
			}
			bufferedEntry = nil
		}
	}

	for {
		n, err := file.Read(buf)
		if n > 0 {
			data := buf[:n]
			allData := append(leftover, data...)
			leftover = nil

			lines := strings.Split(string(allData), "\n")

			for i, line := range lines {
				if i == len(lines)-1 && !strings.HasSuffix(string(allData), "\n") {
					leftover = []byte(line)
					continue
				}

				lineCount++
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					if debug {
						fmt.Printf("Line %d: [EMPTY LINE]\n", lineCount)
					}
					continue
				}

				isNewEntry := logs.IsLikelyNewLogEntry(line)

				if !isNewEntry && bufferedEntry != nil {
					bufferedEntry.Raw = bufferedEntry.Raw + "\n" + trimmed
					bufferedEntry = logs.ParseLogLine(bufferedEntry.Raw)
					if strings.Contains(bufferedEntry.Message, "[sql]") && len(bufferedEntry.Fields) > 0 {
						flushBuffered()
					}
				} else {
					flushBuffered()

					entry := logs.ParseLogLine(trimmed)

					shouldBuffer := (strings.Contains(entry.Message, "[sql]") && len(entry.Fields) == 0) ||
						(entry.Timestamp != "" && len(entry.Fields) == 0 && len(entry.Message) > 0)

					if shouldBuffer {
						bufferedEntry = entry
					} else {
						entryCount++
						if shouldShowEntry(entry, filterLevel, filterText) {
							printLogEntry(entryCount, entry, debug, verbose)
							if csvEntries != nil {
								*csvEntries = append(*csvEntries, entry)
								*csvLineNumbers = append(*csvLineNumbers, entryCount)
							}
						}
					}
				}
			}
		}

		if err != nil {
			break
		}
	}

	flushBuffered()

	fmt.Printf("\nProcessed %d lines from file (%d log entries).\n", lineCount, entryCount)
}

func printLogEntry(lineNum int, entry *logs.LogEntry, debug bool, verbose bool) {
	fmt.Println(entry.Raw)
	// fmt.Println(strings.Repeat("-", 40))

	fmt.Println(fmt.Sprintf("%3d |"+strings.Repeat("-", 80)+"|", lineNum))
	fmt.Printf("    |----- %-20s -----|-- %-6s --|----- %-21s ------|\n", "Timestamp", "Level", "File")
	message := entry.Message
	if len(message) > 120 {
		message = message[:80] + "..." + message[len(message)-40:]
	}
	fmt.Printf("    | %-30s | %-10s | %32s |\n    | %s\n", entry.Timestamp, entry.Level, entry.File[len(entry.File)-min(len(entry.File), 32):], message)
	fmt.Println("    |" + strings.Repeat("-", 80) + "|")
	if len(entry.Fields) > 0 {
		for k, v := range entry.Fields {
			fmt.Printf("    |  %25s | %+v\n", k, v)
		}
	}
	fmt.Println("    |" + strings.Repeat("-", 80) + "|")

	fmt.Println()
}

// shouldShowEntry determines if a log entry should be displayed based on filters
func shouldShowEntry(entry *logs.LogEntry, filterLevel string, filterText string) bool {
	// Filter by level if specified
	if filterLevel != "" {
		if !strings.EqualFold(entry.Level, filterLevel) {
			return false
		}
	}

	// Filter by text if specified
	if filterText != "" {
		if !entry.MatchesSearch(filterText) {
			return false
		}
	}

	return true
}

// writeCSVFile writes all log entries to CSV with dynamic headers for each unique field
func writeCSVFile(writer *csv.Writer, entries []*logs.LogEntry, lineNumbers []int) {
	// Collect all unique field names across all entries
	fieldNamesMap := make(map[string]bool)
	for _, entry := range entries {
		for fieldName := range entry.Fields {
			fieldNamesMap[fieldName] = true
		}
	}

	// Convert to sorted slice for consistent column order
	fieldNames := make([]string, 0, len(fieldNamesMap))
	for fieldName := range fieldNamesMap {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)

	// Build header row
	header := []string{"Line", "Timestamp", "Level", "File", "Message"}
	header = append(header, fieldNames...)

	// Write header
	err := writer.Write(header)
	if err != nil {
		fmt.Printf("Error writing CSV header: %v\n", err)
		return
	}

	// Write each entry
	for i, entry := range entries {
		record := []string{
			fmt.Sprintf("%d", lineNumbers[i]),
			entry.Timestamp,
			entry.Level,
			entry.File,
			entry.Message,
		}

		// Add field values in the same order as headers
		for _, fieldName := range fieldNames {
			if value, ok := entry.Fields[fieldName]; ok {
				record = append(record, value)
			} else {
				record = append(record, "")
			}
		}

		err := writer.Write(record)
		if err != nil {
			fmt.Printf("Error writing CSV record: %v\n", err)
		}
	}
}
