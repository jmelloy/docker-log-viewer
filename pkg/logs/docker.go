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

var (
	timestampPatternRegex = regexp.MustCompile(`^\d{4}[-/]\d{2}[-/]\d{2}|\d{1,2}\s+\w+\s+\d{4}|\d{2}:\d{2}:\d{2}`)
	timestampMonths       = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
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

// hasTimestamp checks if a line starts with a timestamp
func hasTimestamp(line string) bool {
	trimmed := strings.TrimSpace(line)
	// Check for common timestamp patterns at the start
	for _, month := range timestampMonths {
		if strings.HasPrefix(trimmed, month+" ") {
			return true
		}
	}
	// Check for numeric timestamp patterns (YYYY-MM-DD, etc.)
	return timestampPatternRegex.MatchString(trimmed)
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

func (dc *DockerClient) StreamLogs(ctx context.Context, containerID string, logChan chan<- LogMessage) error {
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
		buf := make([]byte, 8192)
		var leftover []byte
		var bufferedLog strings.Builder
		lineCount := 0

		flushLog := func() {
			if bufferedLog.Len() > 0 {
				logText := bufferedLog.String()
				entry := ParseLogLine(logText)
				logChan <- LogMessage{
					ContainerID: containerID,
					Timestamp:   time.Now(),
					Entry:       entry,
				}
				bufferedLog.Reset()
				lineCount++
			}
		}

		for {
			select {
			case <-ctx.Done():
				slog.Info("Container context cancelled, stopping stream", "container_id", containerID[:12])
				flushLog()
				return
			default:
				n, err := reader.Read(buf)
				if n > 0 {
					slog.Debug("Container read bytes from Docker", "container_id", containerID[:12], "bytes", n)
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
					slog.Debug("Container split into lines", "container_id", containerID[:12], "lines", len(lines))

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

						// Check if this line starts with a timestamp
						lineHasTimestamp := hasTimestamp(line)

						if lineHasTimestamp {
							// New log entry - flush any buffered content first
							flushLog()
							bufferedLog.WriteString(trimmed)
						} else {
							// Check if this is a continuation line (indented)
							// Note: we check raw 'line' for whitespace prefix, not 'trimmed',
							// because indentation is the indicator of continuation
							isContinuationLine := bufferedLog.Len() > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"))

							if isContinuationLine {
								// Add to buffer
								bufferedLog.WriteString("\n")
								bufferedLog.WriteString(trimmed)
							} else {
								// Standalone line without timestamp - flush buffer and process
								flushLog()
								entry := ParseLogLine(trimmed)
								logChan <- LogMessage{
									ContainerID: containerID,
									Timestamp:   time.Now(),
									Entry:       entry,
								}
								lineCount++
								sentCount++
							}
						}
					}

					if sentCount > 0 {
						slog.Debug("Container sent messages to logChan", "container_id", containerID[:12], "messages", sentCount)
					}
					if emptyCount > 0 {
						slog.Debug("Container skipped empty lines", "container_id", containerID[:12], "lines", emptyCount)
					}
					if lineCount > 0 && lineCount%100 == 0 {
						slog.Info("Container processed log lines", "container_id", containerID[:12], "lines", lineCount)
					}
				}

				if err == io.EOF {
					flushLog()
					slog.Debug("Container reached EOF, total lines processed", "container_id", containerID[:12], "lines", lineCount)
					return
				}
				if err != nil {
					flushLog()
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
