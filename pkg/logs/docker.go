package logs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	cli *client.Client
}

type Container struct {
	ID    string
	Name  string
	Image string
	Ports []PortMapping
}

type PortMapping struct {
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort"`
	Type        string `json:"type"`
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return &DockerClient{cli: cli}, nil
}

func (dc *DockerClient) ListRunningContainers(ctx context.Context) ([]Container, error) {
	containers, err := dc.cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	result := make([]Container, 0, len(containers))
	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")

		// Extract port mappings
		ports := make([]PortMapping, 0)
		for _, port := range c.Ports {
			ports = append(ports, PortMapping{
				PrivatePort: int(port.PrivatePort),
				PublicPort:  int(port.PublicPort),
				Type:        port.Type,
			})
		}

		result = append(result, Container{
			ID:    c.ID, // Use full ID for StreamLogs
			Name:  name,
			Image: c.Image,
			Ports: ports,
		})
	}
	return result, nil
}

func (dc *DockerClient) StreamLogs(ctx context.Context, containerID string, logChan chan<- LogMessage, onStreamEnd func()) error {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
		Tail:       "10000",
	}

	slog.Info("Starting log stream for container", "container_id", containerID[:12], "tail", options.Tail)

	reader, err := dc.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}

	go func() {
		defer reader.Close()
		if onStreamEnd != nil {
			defer onStreamEnd()
		}
		buf := make([]byte, 8192)
		var leftover []byte
		var bufferedEntry *LogEntry
		lineCount := 0

		flushBuffered := func() {
			if bufferedEntry != nil {
				// Try to send, but don't block if context is cancelled or channel is closed
				select {
				case <-ctx.Done():
					// Context cancelled, don't send
					return
				case logChan <- LogMessage{
					ContainerID: containerID,
					Timestamp:   time.Now(), // Will be updated below if bufferedEntry has timestamp
					Entry:       bufferedEntry,
				}:
					bufferedEntry = nil
					lineCount++
				}
			}
		}

		for {
			select {
			case <-ctx.Done():
				flushBuffered()
				slog.Info("Container context cancelled, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
				return
			default:
				n, err := reader.Read(buf)
				if n > 0 {
					// slog.Debug("Container read bytes from Docker", "container_id", containerID[:12], "bytes", n)
					data := buf[:n]

					cleanedData := make([]byte, 0, len(data))
					i := 0
					for i < len(data) {
						if i+8 <= len(data) && (data[i] == 0 || data[i] == 1 || data[i] == 2) {
							i += 8
						} else {
							cleanedData = append(cleanedData, data[i])
							i++
						}
					}

					allData := append(leftover, cleanedData...)
					leftover = nil

					lines := strings.Split(string(allData), "\n")
					// slog.Debug("Container split into lines", "container_id", containerID[:12], "lines", len(lines))

					emptyCount := 0
					sentCount := 0
					for i, line := range lines {
						if i == len(lines)-1 && !strings.HasSuffix(string(allData), "\n") {
							leftover = []byte(line)
							continue
						}

						trimmed := strings.TrimSpace(line)
						if trimmed == "" {
							emptyCount++
							continue
						}

						// Use ANSI codes and other heuristics to detect new log entries
						isNewEntry := IsLikelyNewLogEntry(line)

						// If this looks like a continuation line and we have a buffered entry, append to it
						if !isNewEntry && bufferedEntry != nil {
							// Combine with the buffered entry
							bufferedEntry.Raw = bufferedEntry.Raw + "\n" + trimmed
							// Re-parse the combined raw text
							bufferedEntry = ParseLogLine(bufferedEntry.Raw)
							// Check if the buffered entry now looks complete (has structured fields)
							// For SQL entries, we need fields. For other entries, we'll flush on next new entry.
							if strings.Contains(bufferedEntry.Message, "[sql]") && len(bufferedEntry.Fields) > 0 {
								flushBuffered()
							}
						} else {
							// Flush any buffered entry first
							flushBuffered()

							entry := ParseLogLine(trimmed)

							// Check if this entry might have continuation lines
							// SQL entries without fields or entries ending with incomplete patterns
							shouldBuffer := (strings.Contains(entry.Message, "[sql]") && len(entry.Fields) == 0) ||
								(entry.Timestamp != "" && len(entry.Fields) == 0 && len(entry.Message) > 0)

							if shouldBuffer {
								// Buffer it, waiting for potential continuation lines
								bufferedEntry = entry
							} else {
								// Send immediately, but check context first
								select {
								case <-ctx.Done():
									return
								case logChan <- LogMessage{
									ContainerID: containerID,
									Timestamp:   time.Now(),
									Entry:       entry,
								}:
									lineCount++
									sentCount++
								}
							}
						}
					}
					// if sentCount > 0 {
					// 	slog.Debug("Container sent messages to logChan", "container_id", containerID[:12], "messages", sentCount)
					// }
					// if emptyCount > 0 {
					// 	slog.Debug("Container skipped empty lines", "container_id", containerID[:12], "lines", emptyCount)
					// }
					if lineCount > 0 && lineCount%100 == 0 {
						slog.Info("Container processed log lines", "container_id", containerID[:12], "lines", lineCount)
					}
				}

				if err == io.EOF {
					flushBuffered()
					slog.Info("Container reached EOF, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
					return
				}
				if err != nil {
					flushBuffered()
					slog.Error("Container log stream error, stopping", "container_id", containerID[:12], "error", err, "linesProcessed", lineCount)
					return
				}
			}
		}
	}()

	return nil
}

func (dc *DockerClient) Close() error {
	if dc.cli != nil {
		return dc.cli.Close()
	}
	return nil
}

type LogMessage struct {
	ContainerID string
	Timestamp   time.Time
	Entry       *LogEntry
}
