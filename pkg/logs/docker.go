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
			ID:    c.ID[:12],
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
		lineCount := 0

		for {
			select {
			case <-ctx.Done():
				slog.Info("Container context cancelled, stopping stream", "container_id", containerID[:12])
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
						if strings.TrimSpace(line) != "" {
							entry := ParseLogLine(line)
							logChan <- LogMessage{
								ContainerID: containerID,
								Timestamp:   time.Now(),
								Entry:       entry,
							}
							lineCount++
							sentCount++
						} else {
							emptyCount++
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
					slog.Debug("Container reached EOF, total lines processed", "container_id", containerID[:12], "lines", lineCount)
					return
				}
				if err != nil {
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
