package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
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
		tail        = flag.Int("tail", 100, "Number of recent log lines to show (for Docker)")
		follow      = flag.Bool("follow", false, "Follow logs in real-time (Docker only)")
	)
	flag.Parse()

	// Set up logging
	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if *containerID == "" && *logFile == "" {
		fmt.Println("Usage:")
		fmt.Println("  Read from Docker container:")
		fmt.Println("    go run cmd/test_parser.go -container <container_id_or_name> [-tail 100] [-follow]")
		fmt.Println("  Read from log file:")
		fmt.Println("    go run cmd/test_parser.go -file <log_file_path>")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -debug     Enable debug output")
		fmt.Println("  -verbose   Enable verbose output")
		fmt.Println("  -tail      Number of recent log lines to show (Docker only, default: 100)")
		fmt.Println("  -follow    Follow logs in real-time (Docker only)")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run cmd/test_parser.go -container glue-api-glue-api-1 -debug")
		fmt.Println("  go run cmd/test_parser.go -file /var/log/app.log -verbose")
		fmt.Println("  go run cmd/test_parser.go -container myapp -follow -tail 50")
		os.Exit(1)
	}

	if *containerID != "" && *logFile != "" {
		fmt.Println("Error: Cannot specify both container and file. Choose one.")
		os.Exit(1)
	}

	if *containerID != "" {
		readFromDockerContainer(*containerID, *tail, *follow, *debug, *verbose)
	} else {
		readFromLogFile(*logFile, *debug, *verbose)
	}
}

func readFromDockerContainer(containerID string, tail int, follow bool, debug bool, verbose bool) {
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

	// List containers to help with debugging
	if debug {
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
	}

	// Create log channel
	logChan := make(chan logs.LogMessage, 1000)

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
			printLogEntry(lineCount, logMsg.Entry, debug, verbose)

			if !follow && lineCount >= tail {
				fmt.Printf("\nReached tail limit (%d lines). Use -follow to continue streaming.\n", tail)
				return
			}
		case <-time.After(5 * time.Second):
			if !follow {
				fmt.Printf("\nNo more logs available after 5 seconds.\n")
				return
			}
		}
	}
}

func readFromLogFile(filePath string, debug bool, verbose bool) {
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
			printLogEntry(entryCount, bufferedEntry, debug, verbose)
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

				fmt.Println(line)
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
						printLogEntry(entryCount, entry, debug, verbose)
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
