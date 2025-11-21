package handlers

import (
	"context"
	"sync"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/websocket"
)

// WebApp represents the main application state
type WebApp struct {
	Docker              *logs.DockerClient
	LogStore            *logstore.LogStore
	Containers          []logs.Container
	ContainerIDNames    map[string]string
	ContainerMutex      sync.RWMutex
	Clients             map[*Client]bool
	ClientsMutex        sync.RWMutex
	LogChan             chan logs.LogMessage
	BatchChan           chan struct{}
	LogBatch            []logs.LogMessage
	BatchMutex          sync.Mutex
	Ctx                 context.Context
	Cancel              context.CancelFunc
	Upgrader            websocket.Upgrader
	Store               *store.Store
	LastTimestamps      map[string]time.Time
	LastTimestampsMutex sync.RWMutex
	ShutdownOnce        sync.Once
	ActiveStreams       map[string]bool
	ActiveStreamsMutex  sync.RWMutex
}

type ClientFilter struct {
	SelectedContainers []string           `json:"selectedContainers"`
	SelectedLevels     []string           `json:"selectedLevels"`
	SearchQuery        string             `json:"searchQuery"`
	TraceFilters       []TraceFilterValue `json:"traceFilters"`
}

type TraceFilterValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Client struct {
	Conn   *websocket.Conn
	Filter ClientFilter
	Mu     sync.RWMutex
}

type RetentionInfo struct {
	Type  string `json:"type"`  // "count" or "time"
	Value int    `json:"value"` // number of logs or seconds
}

type ContainersUpdateMessage struct {
	Containers      []logs.Container         `json:"containers"`
	PortToServerMap map[int]string           `json:"portToServerMap"`
	LogCounts       map[string]int           `json:"logCounts"`
	Retentions      map[string]RetentionInfo `json:"retentions"`
}

type LogWSMessage struct {
	ContainerID string         `json:"containerId"`
	Timestamp   time.Time      `json:"timestamp"`
	Entry       *logs.LogEntry `json:"entry"`
}
