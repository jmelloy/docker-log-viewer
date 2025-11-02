package logstore

import (
	"container/list"
	"log/slog"
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
	byContainer map[string]*list.List            // container_id -> list of elements
	byField     map[string]map[string]*list.List // field_name -> field_value -> list of elements

	// Configuration
	maxMessages int
	maxAge      time.Duration

	// Element tracking
	messageCount int

	// Per-container retention settings
	containerRetention map[string]ContainerRetentionPolicy
}

// ContainerRetentionPolicy defines retention for a specific container
type ContainerRetentionPolicy struct {
	Type  string // "count" or "time"
	Value int    // number of logs or seconds
}

// NewLogStore creates a new log store
func NewLogStore(maxMessages int, maxAge time.Duration) *LogStore {
	return &LogStore{
		messages:           list.New(),
		byContainer:        make(map[string]*list.List),
		byField:            make(map[string]map[string]*list.List),
		maxMessages:        maxMessages,
		maxAge:             maxAge,
		messageCount:       0,
		containerRetention: make(map[string]ContainerRetentionPolicy),
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

	// Check if this container has a retention policy, otherwise use default per-container limit
	if _, exists := ls.containerRetention[msg.ContainerID]; exists {
		ls.applyContainerRetention(msg.ContainerID)
	} else {
		// Apply default per-container limit (maxMessages is per-container, not global)
		containerList := ls.byContainer[msg.ContainerID]
		if containerList != nil && containerList.Len() > ls.maxMessages {
			// Remove oldest message from this container
			e := containerList.Back()
			if e != nil {
				elem := e.Value.(*list.Element)
				msg := elem.Value.(*LogMessage)
				ls.removeMessage(elem, msg)
			}
		}
	}

	// Evict messages older than maxAge
	ls.evictExpired()
}

// evictOldest removes the oldest message from a specific container
func (ls *LogStore) evictOldestFromContainer(containerID string) {
	containerList := ls.byContainer[containerID]
	if containerList == nil {
		return
	}

	e := containerList.Back()
	if e != nil {
		elem := e.Value.(*list.Element)
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
		// Inline GetRecent logic to avoid deadlock
		results := make([]*LogMessage, 0, min(limit, ls.messageCount))
		count := 0

		for e := ls.messages.Front(); e != nil && count < limit; e = e.Next() {
			results = append(results, e.Value.(*LogMessage))
			count++
		}

		return results
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

// CountByContainer returns the number of messages for a specific container
func (ls *LogStore) CountByContainer(containerID string) int {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	containerList := ls.byContainer[containerID]
	if containerList == nil {
		return 0
	}
	return containerList.Len()
}

// SetMaxMessages updates the maximum message limit per container
func (ls *LogStore) SetMaxMessages(max int) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.maxMessages = max

	// Evict excess messages from each container
	for containerID, containerList := range ls.byContainer {
		for containerList.Len() > ls.maxMessages {
			ls.evictOldestFromContainer(containerID)
		}
	}
}

// FilterOptions represents all filtering criteria for log messages
type FilterOptions struct {
	ContainerIDs []string // Empty means all containers
	Levels       []string // Empty means all levels
	SearchTerms  []string // All terms must match (AND)
	FieldFilters []FieldFilter
}

// Filter returns messages matching all filter criteria with a limit
func (ls *LogStore) Filter(opts FilterOptions, limit int) []*LogMessage {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	results := make([]*LogMessage, 0, limit)
	count := 0

	// Priority: FieldFilters > Single Container > Multiple Containers > All Messages

	// If field filters specified, use the smallest field index
	if len(opts.FieldFilters) > 0 {
		smallestSize := -1
		var fieldIndexList *list.List
		for _, filter := range opts.FieldFilters {
			if fieldMap := ls.byField[filter.Name]; fieldMap != nil {
				if valueList := fieldMap[filter.Value]; valueList != nil {
					size := valueList.Len()
					if smallestSize == -1 || size < smallestSize {
						smallestSize = size
						fieldIndexList = valueList
					}
				} else {
					// Value not found, no results possible
					return results
				}
			} else {
				// Field not found, no results possible
				return results
			}
		}

		// Use field index for iteration
		for e := fieldIndexList.Front(); e != nil && count < limit; e = e.Next() {
			elem := e.Value.(*list.Element)
			msg := elem.Value.(*LogMessage)
			if ls.matchesFilterOptions(msg, opts) {
				results = append(results, msg)
				count++
			}
		}
		return results
	}

	// Single container - use container index
	if len(opts.ContainerIDs) == 1 {
		containerList := ls.byContainer[opts.ContainerIDs[0]]
		if containerList == nil {
			return results
		}

		for e := containerList.Front(); e != nil && count < limit; e = e.Next() {
			elem := e.Value.(*list.Element)
			msg := elem.Value.(*LogMessage)
			if ls.matchesFilterOptions(msg, opts) {
				results = append(results, msg)
				count++
			}
		}
		return results
	}

	// Multiple containers - iterate through each container's index
	if len(opts.ContainerIDs) > 1 {
		for _, containerID := range opts.ContainerIDs {
			containerList := ls.byContainer[containerID]
			if containerList == nil {
				continue
			}

			for e := containerList.Front(); e != nil && count < limit; e = e.Next() {
				elem := e.Value.(*list.Element)
				msg := elem.Value.(*LogMessage)
				if ls.matchesFilterOptions(msg, opts) {
					results = append(results, msg)
					count++
					if count >= limit {
						return results
					}
				}
			}
		}
		return results
	}

	// No indexes to use, search main list
	for e := ls.messages.Front(); e != nil && count < limit; e = e.Next() {
		msg := e.Value.(*LogMessage)
		if ls.matchesFilterOptions(msg, opts) {
			results = append(results, msg)
			count++
		}
	}

	return results
}

// matchesFilterOptions checks if a message matches all filter criteria
func (ls *LogStore) matchesFilterOptions(msg *LogMessage, opts FilterOptions) bool {
	// Container filter
	if len(opts.ContainerIDs) > 0 {
		found := false
		for _, containerID := range opts.ContainerIDs {
			if msg.ContainerID == containerID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Level filter
	if len(opts.Levels) > 0 {
		level := msg.Fields["_level"]
		if level == "" {
			// No level parsed - check if NONE is selected
			found := false
			for _, l := range opts.Levels {
				if l == "NONE" {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else {
			// Has a level - check if it matches (case-insensitive)
			levelUpper := strings.ToUpper(level)
			found := false
			for _, l := range opts.Levels {
				if strings.ToUpper(l) == levelUpper {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Search terms filter - AND multiple terms together
	if len(opts.SearchTerms) > 0 {
		for _, term := range opts.SearchTerms {
			query := strings.ToLower(term)
			found := false

			// Search in message
			if strings.Contains(strings.ToLower(msg.Message), query) {
				found = true
			}

			// Search in raw log
			if !found {
				if raw, ok := msg.Fields["_raw"]; ok {
					if strings.Contains(strings.ToLower(raw), query) {
						found = true
					}
				}
			}

			// Search in fields
			if !found {
				for key, value := range msg.Fields {
					if strings.Contains(strings.ToLower(key), query) || strings.Contains(strings.ToLower(value), query) {
						found = true
						break
					}
				}
			}

			// If any term is not found, the log doesn't match (AND logic)
			if !found {
				return false
			}
		}
	}

	// Field filters - all must match
	for _, filter := range opts.FieldFilters {
		if msg.Fields[filter.Name] != filter.Value {
			return false
		}
	}

	return true
}

// SetContainerRetention sets retention policy for a specific container
func (ls *LogStore) SetContainerRetention(containerID string, policy ContainerRetentionPolicy) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.containerRetention[containerID] = policy

	// Apply retention immediately
	ls.applyContainerRetention(containerID)
}

// RemoveContainerRetention removes retention policy for a container
func (ls *LogStore) RemoveContainerRetention(containerID string) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	delete(ls.containerRetention, containerID)
}

// applyContainerRetention applies retention policy for a specific container
// Must be called with lock held
func (ls *LogStore) applyContainerRetention(containerID string) {
	policy, exists := ls.containerRetention[containerID]
	if !exists {
		return
	}

	containerList := ls.byContainer[containerID]
	if containerList == nil {
		return
	}

	switch policy.Type {
	case "count":
		// Remove oldest logs exceeding count limit
		count := containerList.Len()
		toRemove := count - policy.Value

		if toRemove > 0 {
			// Iterate from back (oldest) and remove
			for i := 0; i < toRemove; i++ {
				e := containerList.Back()
				if e != nil {
					elem := e.Value.(*list.Element)
					msg := elem.Value.(*LogMessage)
					ls.removeMessage(elem, msg)
				}
			}
		}
		if toRemove > 1 {
			slog.Debug("removed count", "containerID", containerID, "toRemove", toRemove)
		}
	case "time":
		// Remove logs older than specified seconds, but always keep at least 100
		cutoff := time.Now().Add(-time.Duration(policy.Value) * time.Second)

		count := containerList.Len()
		minToKeep := 100

		removedCount := 0
		for e := containerList.Back(); e != nil && count > minToKeep; e = e.Prev() {
			elem := e.Value.(*list.Element)
			msg := elem.Value.(*LogMessage)

			if msg.Timestamp.Before(cutoff) {
				ls.removeMessage(elem, msg)
				count--
				removedCount++
			}
		}
		slog.Info("removed count based on time", "containerID", containerID, "removedCount", removedCount)
	}

}
