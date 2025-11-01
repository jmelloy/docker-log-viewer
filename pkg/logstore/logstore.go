package logstore

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

// LogMessage represents a single log entry
type LogMessage struct {
	Timestamp   time.Time
	ContainerID string
	Message     string
	Fields      map[string]string // Dynamic fields like request_id, etc.
}

// LogStore provides efficient storage and search for log messages
type LogStore struct {
	mu sync.RWMutex

	// Main storage - doubly linked list for efficient insertion/eviction
	messages *list.List

	// Indexes for fast lookups
	byContainer map[string]*list.List                   // container_id -> list of elements
	byField     map[string]map[string]*list.List        // field_name -> field_value -> list of elements

	// Configuration
	maxMessages int
	maxAge      time.Duration

	// Element tracking
	messageCount int
}

// NewLogStore creates a new log store
func NewLogStore(maxMessages int, maxAge time.Duration) *LogStore {
	return &LogStore{
		messages:     list.New(),
		byContainer:  make(map[string]*list.List),
		byField:      make(map[string]map[string]*list.List),
		maxMessages:  maxMessages,
		maxAge:       maxAge,
		messageCount: 0,
	}
}

// Add inserts a new log message and maintains size/age constraints
func (ls *LogStore) Add(msg *LogMessage) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Add to main list (most recent at front)
	elem := ls.messages.PushFront(msg)
	ls.messageCount++

	// Index by container
	if ls.byContainer[msg.ContainerID] == nil {
		ls.byContainer[msg.ContainerID] = list.New()
	}
	ls.byContainer[msg.ContainerID].PushFront(elem)

	// Index by dynamic fields
	for k, v := range msg.Fields {
		if ls.byField[k] == nil {
			ls.byField[k] = make(map[string]*list.List)
		}
		if ls.byField[k][v] == nil {
			ls.byField[k][v] = list.New()
		}
		ls.byField[k][v].PushFront(elem)
	}

	// Evict old messages based on count
	if ls.messageCount > ls.maxMessages {
		ls.evictOldest()
	}

	// Evict messages older than maxAge
	ls.evictExpired()
}

// evictOldest removes the oldest message
func (ls *LogStore) evictOldest() {
	elem := ls.messages.Back()
	if elem != nil {
		msg := elem.Value.(*LogMessage)
		ls.removeMessage(elem, msg)
	}
}

// evictExpired removes messages older than maxAge
func (ls *LogStore) evictExpired() {
	cutoff := time.Now().Add(-ls.maxAge)

	for {
		elem := ls.messages.Back()
		if elem == nil {
			break
		}

		msg := elem.Value.(*LogMessage)
		if msg.Timestamp.After(cutoff) {
			break
		}

		ls.removeMessage(elem, msg)
	}
}

// removeMessage removes a message from all indexes
func (ls *LogStore) removeMessage(elem *list.Element, msg *LogMessage) {
	// Remove from main list
	ls.messages.Remove(elem)
	ls.messageCount--

	// Remove from container index
	if containerList := ls.byContainer[msg.ContainerID]; containerList != nil {
		for e := containerList.Front(); e != nil; e = e.Next() {
			if e.Value.(*list.Element) == elem {
				containerList.Remove(e)
				break
			}
		}
		if containerList.Len() == 0 {
			delete(ls.byContainer, msg.ContainerID)
		}
	}

	// Remove from field indexes
	for k, v := range msg.Fields {
		if fieldMap := ls.byField[k]; fieldMap != nil {
			if valueList := fieldMap[v]; valueList != nil {
				for e := valueList.Front(); e != nil; e = e.Next() {
					if e.Value.(*list.Element) == elem {
						valueList.Remove(e)
						break
					}
				}
				if valueList.Len() == 0 {
					delete(fieldMap, v)
				}
			}
			if len(fieldMap) == 0 {
				delete(ls.byField, k)
			}
		}
	}
}

// SearchByContainer returns all messages for a specific container
func (ls *LogStore) SearchByContainer(containerID string, limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	containerList := ls.byContainer[containerID]
	if containerList == nil {
		return nil
	}

	results := make([]*LogMessage, 0, min(limit, containerList.Len()))
	count := 0

	for e := containerList.Front(); e != nil && count < limit; e = e.Next() {
		elem := e.Value.(*list.Element)
		results = append(results, elem.Value.(*LogMessage))
		count++
	}

	return results
}

// SearchByField returns messages matching a specific field value
func (ls *LogStore) SearchByField(fieldName, fieldValue string, limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	fieldMap := ls.byField[fieldName]
	if fieldMap == nil {
		return nil
	}

	valueList := fieldMap[fieldValue]
	if valueList == nil {
		return nil
	}

	results := make([]*LogMessage, 0, min(limit, valueList.Len()))
	count := 0

	for e := valueList.Front(); e != nil && count < limit; e = e.Next() {
		elem := e.Value.(*list.Element)
		results = append(results, elem.Value.(*LogMessage))
		count++
	}

	return results
}

// SearchByText performs a substring search in message text
func (ls *LogStore) SearchByText(substring string, limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	results := make([]*LogMessage, 0, limit)
	count := 0

	for e := ls.messages.Front(); e != nil && count < limit; e = e.Next() {
		msg := e.Value.(*LogMessage)
		if strings.Contains(msg.Message, substring) {
			results = append(results, msg)
			count++
		}
	}

	return results
}

// FieldFilter represents a single field constraint
type FieldFilter struct {
	Name  string
	Value string
}

// SearchByFields returns messages matching ALL specified field filters (AND operation)
func (ls *LogStore) SearchByFields(filters []FieldFilter, limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	if len(filters) == 0 {
		return ls.GetRecent(limit)
	}

	// Start with the smallest index for efficiency
	// Find which field has the fewest matches
	smallestIdx := 0
	smallestSize := -1

	for i, filter := range filters {
		if fieldMap := ls.byField[filter.Name]; fieldMap != nil {
			if valueList := fieldMap[filter.Value]; valueList != nil {
				size := valueList.Len()
				if smallestSize == -1 || size < smallestSize {
					smallestSize = size
					smallestIdx = i
				}
			} else {
				// Value not found, no results possible
				return nil
			}
		} else {
			// Field not found, no results possible
			return nil
		}
	}

	// Iterate through smallest set and check other constraints
	startFilter := filters[smallestIdx]
	valueList := ls.byField[startFilter.Name][startFilter.Value]

	results := make([]*LogMessage, 0, min(limit, valueList.Len()))
	count := 0

	for e := valueList.Front(); e != nil && count < limit; e = e.Next() {
		elem := e.Value.(*list.Element)
		msg := elem.Value.(*LogMessage)

		// Check if message matches all other filters
		if ls.matchesAllFilters(msg, filters) {
			results = append(results, msg)
			count++
		}
	}

	return results
}

// matchesAllFilters checks if a message matches all field filters
func (ls *LogStore) matchesAllFilters(msg *LogMessage, filters []FieldFilter) bool {
	for _, filter := range filters {
		if msg.Fields[filter.Name] != filter.Value {
			return false
		}
	}
	return true
}

// SearchCriteria represents multiple search criteria
type SearchCriteria struct {
	ContainerID string        // Optional: filter by container
	Fields      []FieldFilter // Optional: AND filters for fields
	TextSearch  string        // Optional: substring in message
	After       *time.Time    // Optional: messages after this time
	Before      *time.Time    // Optional: messages before this time
}

// SearchComplex performs a search with multiple criteria combined
func (ls *LogStore) SearchComplex(criteria SearchCriteria, limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	results := make([]*LogMessage, 0, limit)
	count := 0

	// Determine starting point for iteration
	var startList *list.List
	var useMainList bool

	// If container specified, start from container index
	if criteria.ContainerID != "" {
		startList = ls.byContainer[criteria.ContainerID]
		if startList == nil {
			return nil
		}
	} else if len(criteria.Fields) > 0 {
		// Start from smallest field index
		smallestSize := -1
		for _, filter := range criteria.Fields {
			if fieldMap := ls.byField[filter.Name]; fieldMap != nil {
				if valueList := fieldMap[filter.Value]; valueList != nil {
					size := valueList.Len()
					if smallestSize == -1 || size < smallestSize {
						smallestSize = size
						startList = valueList
					}
				} else {
					return nil
				}
			} else {
				return nil
			}
		}
	} else {
		// No indexes to use, search main list
		useMainList = true
	}

	// Iterate and apply all filters
	if useMainList {
		for e := ls.messages.Front(); e != nil && count < limit; e = e.Next() {
			msg := e.Value.(*LogMessage)
			if ls.matchesCriteria(msg, criteria) {
				results = append(results, msg)
				count++
			}
		}
	} else {
		for e := startList.Front(); e != nil && count < limit; e = e.Next() {
			elem := e.Value.(*list.Element)
			msg := elem.Value.(*LogMessage)
			if ls.matchesCriteria(msg, criteria) {
				results = append(results, msg)
				count++
			}
		}
	}

	return results
}

// matchesCriteria checks if a message matches all search criteria
func (ls *LogStore) matchesCriteria(msg *LogMessage, criteria SearchCriteria) bool {
	// Check container
	if criteria.ContainerID != "" && msg.ContainerID != criteria.ContainerID {
		return false
	}

	// Check field filters
	for _, filter := range criteria.Fields {
		if msg.Fields[filter.Name] != filter.Value {
			return false
		}
	}

	// Check text search
	if criteria.TextSearch != "" && !strings.Contains(msg.Message, criteria.TextSearch) {
		return false
	}

	// Check time range
	if criteria.After != nil && msg.Timestamp.Before(*criteria.After) {
		return false
	}
	if criteria.Before != nil && msg.Timestamp.After(*criteria.Before) {
		return false
	}

	return true
}

// GetRecent returns the N most recent messages
func (ls *LogStore) GetRecent(limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	results := make([]*LogMessage, 0, min(limit, ls.messageCount))
	count := 0

	for e := ls.messages.Front(); e != nil && count < limit; e = e.Next() {
		results = append(results, e.Value.(*LogMessage))
		count++
	}

	return results
}

// Count returns the current number of messages in the store
func (ls *LogStore) Count() int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.messageCount
}

// SetMaxMessages updates the maximum message limit
func (ls *LogStore) SetMaxMessages(max int) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.maxMessages = max

	// Evict excess messages
	for ls.messageCount > ls.maxMessages {
		ls.evictOldest()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
