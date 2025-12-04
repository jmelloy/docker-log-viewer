package logs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type ContainerMessage struct {
	ContainerID string
	Timestamp   time.Time
	Entry       *LogEntry
}

var dockerTimestampRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(.*)$`)

func parseDockerTimestamp(line string) (time.Time, string) {
	matches := dockerTimestampRegex.FindStringSubmatch(line)
	if matches == nil {
		return time.Time{}, line
	}
	ts, err := time.Parse(time.RFC3339Nano, matches[1])
	if err != nil {
		return time.Time{}, line
	}
	return ts, matches[2]
}

type DockerClient struct {
	cli *client.Client
}

type Container struct {
	ID      string
	Name    string
	Image   string
	Ports   []PortMapping
	Project string // Docker Compose project name
	Service string // Docker Compose service name
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

		// Extract Docker Compose project and service names from labels
		project := ""
		service := ""
		if c.Labels != nil {
			if p, ok := c.Labels["com.docker.compose.project"]; ok {
				project = p
			}
			if s, ok := c.Labels["com.docker.compose.service"]; ok {
				service = s
			}
		}

		result = append(result, Container{
			ID:      c.ID, // Use full ID for StreamLogs
			Name:    name,
			Image:   c.Image,
			Ports:   ports,
			Project: project,
			Service: service,
		})
	}
	return result, nil
}

func (dc *DockerClient) StreamLogs(ctx context.Context, containerID string, logChan chan<- ContainerMessage, onStreamEnd func()) error {
	return dc.StreamLogsSince(ctx, containerID, logChan, onStreamEnd, time.Time{})
}

func (dc *DockerClient) StreamLogsSince(ctx context.Context, containerID string, logChan chan<- ContainerMessage, onStreamEnd func(), since time.Time) error {
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
	}

	if since.IsZero() {
		options.Since = time.Now().Add(-15 * time.Minute).Format(time.RFC3339Nano)
		slog.Info("Starting log stream for container", "container_id", containerID[:12], "since", options.Since)
	} else {
		options.Since = since.Format(time.RFC3339Nano)
		slog.Info("Resuming log stream for container", "container_id", containerID[:12], "since", options.Since)
	}

	reader, err := dc.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}

	go func() {
		channelClosed := false
		defer func() {
			if r := recover(); r != nil {
				// Channel was closed, this is expected during shutdown
				channelClosed = true
				slog.Debug("Recovered from panic in log stream (likely channel closed)", "container_id", containerID[:12], "panic", r)
			}
		}()
		defer reader.Close()
		if onStreamEnd != nil {
			defer onStreamEnd()
		}
		buf := make([]byte, 8192)
		var leftover []byte
		var bufferedEntry *LogEntry
		var bufferedTimestamp time.Time
		lineCount := 0

		// safeSend attempts to send a message to the channel, handling closed channel gracefully
		// Returns true if sent successfully, false if context cancelled or channel closed
		safeSend := func(msg ContainerMessage) bool {
			if channelClosed {
				return false
			}
			// Use a closure with recover to catch panics from closed channel
			var sent bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Channel was closed, mark it and return false
						channelClosed = true
						sent = false
					}
				}()
				select {
				case <-ctx.Done():
					// Context cancelled, don't send
					sent = false
				case logChan <- msg:
					// Successfully sent
					sent = true
				default:
					// Channel is full, don't block
					sent = false
				}
			}()
			return sent
		}

		flushBuffered := func() {
			if bufferedEntry != nil {
				ts := bufferedTimestamp
				if ts.IsZero() {
					ts = time.Now()
				}
				// Try to send, but don't block if context is cancelled or channel is closed
				if safeSend(ContainerMessage{
					ContainerID: containerID,
					Timestamp:   ts,
					Entry:       bufferedEntry,
				}) {
					bufferedEntry = nil
					bufferedTimestamp = time.Time{}
					lineCount++
				}
			}
		}

		for {
			if channelClosed {
				slog.Info("Container log channel closed, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
				return
			}
			select {
			case <-ctx.Done():
				flushBuffered()
				if channelClosed {
					slog.Info("Container log channel closed during flush, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
				} else {
					slog.Info("Container context cancelled, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
				}
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

						// Parse Docker timestamp prefix (format: 2024-12-04T10:30:00.123456789Z <log line>)
						dockerTs, logContent := parseDockerTimestamp(trimmed)
						trimmed = logContent

						// Use ANSI codes and other heuristics to detect new log entries
						isNewEntry := IsLikelyNewLogEntry(trimmed)

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

							// Use Docker timestamp if available, otherwise fall back to now
							ts := dockerTs
							if ts.IsZero() {
								ts = time.Now()
							}

							// Check if this entry might have continuation lines
							// Only buffer SQL entries without fields (they may have multiline SQL)
							// Don't buffer entries just because they lack a timestamp - most log formats work fine without
							shouldBuffer := strings.Contains(entry.Message, "[sql]") && len(entry.Fields) == 0

							if shouldBuffer {
								// Buffer it, waiting for potential continuation lines
								bufferedEntry = entry
								bufferedTimestamp = ts
							} else {
								// Send immediately, but check context first
								if safeSend(ContainerMessage{
									ContainerID: containerID,
									Timestamp:   ts,
									Entry:       entry,
								}) {
									lineCount++
									sentCount++
								} else {
									// Context cancelled or channel closed, exit
									return
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
					if channelClosed {
						slog.Info("Container log channel closed during EOF flush, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
					} else {
						slog.Info("Container reached EOF, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
					}
					return
				}
				if err != nil {
					flushBuffered()
					if channelClosed {
						slog.Info("Container log channel closed during error flush, stopping stream", "container_id", containerID[:12], "linesProcessed", lineCount)
					} else {
						slog.Error("Container log stream error, stopping", "container_id", containerID[:12], "error", err, "linesProcessed", lineCount)
					}
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
