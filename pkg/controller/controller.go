package controller

import (
	"context"
	"net/http"
	"sync"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/schema"
	"github.com/gorilla/websocket"
)

// Controller manages HTTP request handling for the Docker log viewer
type Controller struct {
	docker              *logs.DockerClient
	logStore            *logstore.LogStore
	containers          []logs.Container
	containerIDNames    map[string]string
	containerMutex      sync.RWMutex
	clients             map[*Client]bool
	clientsMutex        sync.RWMutex
	logChan             chan logs.LogMessage
	batchChan           chan struct{}
	logBatch            []logs.LogMessage
	batchMutex          sync.Mutex
	ctx                 context.Context
	cancel              context.CancelFunc
	upgrader            websocket.Upgrader
	store               *store.Store
	lastTimestamps      map[string]time.Time
	lastTimestampsMutex sync.RWMutex
	shutdownOnce        sync.Once
	activeStreams       map[string]bool
	activeStreamsMutex  sync.RWMutex
	decoder             *schema.Decoder
}

// Client represents a WebSocket client connection
type Client struct {
	conn   *websocket.Conn
	filter ClientFilter
	mu     sync.RWMutex
}

// ClientFilter holds filter criteria for a client
type ClientFilter struct {
	SelectedContainers []string           `json:"selectedContainers"`
	SelectedLevels     []string           `json:"selectedLevels"`
	SearchQuery        string             `json:"searchQuery"`
	TraceFilters       []TraceFilterValue `json:"traceFilters"`
}

// TraceFilterValue represents a trace filter
type TraceFilterValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// NewController creates a new Controller instance
func NewController(
	docker *logs.DockerClient,
	logStore *logstore.LogStore,
	store *store.Store,
	ctx context.Context,
	cancel context.CancelFunc,
	logChan chan logs.LogMessage,
) *Controller {
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)

	return &Controller{
		docker:           docker,
		logStore:         logStore,
		store:            store,
		ctx:              ctx,
		cancel:           cancel,
		logChan:          logChan,
		batchChan:        make(chan struct{}),
		logBatch:         make([]logs.LogMessage, 0, 100),
		containerIDNames: make(map[string]string),
		clients:          make(map[*Client]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		lastTimestamps: make(map[string]time.Time),
		activeStreams:  make(map[string]bool),
		decoder:        decoder,
	}
}

// SetContainers updates the controller's container list
func (c *Controller) SetContainers(containers []logs.Container) {
	c.containerMutex.Lock()
	defer c.containerMutex.Unlock()
	c.containers = containers
	for _, container := range containers {
		c.containerIDNames[container.ID] = container.Name
	}
}

// GetContainers returns the current container list
func (c *Controller) GetContainers() []logs.Container {
	c.containerMutex.RLock()
	defer c.containerMutex.RUnlock()
	return c.containers
}

// Shutdown safely cancels context and closes channels
func (c *Controller) Shutdown() {
	c.shutdownOnce.Do(func() {
		c.cancel()
		time.Sleep(300 * time.Millisecond)
		close(c.logChan)
	})
}
