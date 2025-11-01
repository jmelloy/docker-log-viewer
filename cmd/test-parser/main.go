package main

import (
	"bufio"
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
	err = dockerClient.StreamLogs(ctx, containerID, logChan)
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

	scanner := bufio.NewScanner(file)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			if debug {
				fmt.Printf("Line %d: [EMPTY LINE]\n", lineCount)
			}
			continue
		}

		entry := logs.ParseLogLine(line)
		printLogEntry(lineCount, entry, debug, verbose)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nProcessed %d lines from file.\n", lineCount)
}

func printLogEntry(lineNum int, entry *logs.LogEntry, debug bool, verbose bool) {
	fmt.Printf("\n--- Line %d ---\n", lineNum)

	if debug {
		fmt.Printf("Raw: %s\n", entry.Raw)
		fmt.Println(strings.Repeat("-", 40))
	}

	fmt.Printf("IsJSON: %v\n", entry.IsJSON)
	fmt.Printf("Timestamp: %s\n", entry.Timestamp)
	fmt.Printf("Level: %s\n", entry.Level)
	fmt.Printf("File: %s\n", entry.File)
	fmt.Printf("Message: %s\n", entry.Message)

	if len(entry.Fields) > 0 {
		fmt.Printf("Fields (%d):\n", len(entry.Fields))
		for k, v := range entry.Fields {
			if verbose || len(v) < 200 {
				fmt.Printf("  %s = %s\n", k, v)
			} else {
				fmt.Printf("  %s = %s... (truncated, %d chars)\n", k, v[:200], len(v))
			}
		}
	}

	if entry.IsJSON && len(entry.JSONFields) > 0 {
		fmt.Printf("JSON Fields (%d):\n", len(entry.JSONFields))
		for k, v := range entry.JSONFields {
			if verbose {
				fmt.Printf("  %s = %+v\n", k, v)
			} else {
				fmt.Printf("  %s = %T\n", k, v)
			}
		}
	}

	if debug {
		fmt.Printf("Formatted: %s\n", entry.FormattedString())
	}

	fmt.Println(strings.Repeat("-", 40))
}
