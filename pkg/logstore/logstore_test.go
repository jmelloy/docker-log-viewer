package logstore

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewLogStore(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.messageCount != 0 {
		t.Errorf("Expected initial count 0, got %d", store.messageCount)
	}
	if store.maxMessages != 100 {
		t.Errorf("Expected maxMessages 100, got %d", store.maxMessages)
	}
	if store.maxAge != 1*time.Hour {
		t.Errorf("Expected maxAge 1h, got %v", store.maxAge)
	}
}

func TestAddAndGetRecent(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	msg1 := &LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Test message 1",
		Fields:      map[string]string{"request_id": "req1"},
	}

	msg2 := &LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container2",
		Message:     "Test message 2",
		Fields:      map[string]string{"request_id": "req2"},
	}

	store.Add(msg1)
	store.Add(msg2)

	if store.Count() != 2 {
		t.Errorf("Expected count 2, got %d", store.Count())
	}

	recent := store.GetRecent(10)
	if len(recent) != 2 {
		t.Errorf("Expected 2 recent messages, got %d", len(recent))
	}

	// Most recent should be first (msg2)
	if recent[0].Message != "Test message 2" {
		t.Errorf("Expected most recent message first, got %s", recent[0].Message)
	}
}

func TestEvictionByCount(t *testing.T) {
	store := NewLogStore(5, 1*time.Hour)

	// Add 10 messages
	for i := 0; i < 10; i++ {
		msg := &LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     fmt.Sprintf("Message %d", i),
			Fields:      map[string]string{"index": fmt.Sprintf("%d", i)},
		}
		store.Add(msg)
	}

	// Should only have 5 messages (most recent)
	if store.Count() != 5 {
		t.Errorf("Expected count 5 after eviction, got %d", store.Count())
	}

	recent := store.GetRecent(10)
	if len(recent) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(recent))
	}

	// Should have messages 9, 8, 7, 6, 5 (most recent first)
	if recent[0].Message != "Message 9" {
		t.Errorf("Expected 'Message 9', got %s", recent[0].Message)
	}
	if recent[4].Message != "Message 5" {
		t.Errorf("Expected 'Message 5' as oldest, got %s", recent[4].Message)
	}
}

func TestEvictionByAge(t *testing.T) {
	store := NewLogStore(100, 100*time.Millisecond)

	baseTime := time.Now()

	// Add old messages (200ms before base time)
	oldTime := baseTime.Add(-200 * time.Millisecond)
	for i := 0; i < 3; i++ {
		msg := &LogMessage{
			Timestamp:   oldTime,
			ContainerID: "container1",
			Message:     fmt.Sprintf("Old message %d", i),
			Fields:      map[string]string{},
		}
		store.Add(msg)
	}

	// Sleep to ensure enough time passes that the old messages should be expired
	time.Sleep(150 * time.Millisecond)

	// Add a new message which should trigger eviction of old messages
	newMsg := &LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "New message",
		Fields:      map[string]string{},
	}
	store.Add(newMsg)

	// Old messages should be evicted (they are now >100ms old)
	if store.Count() != 1 {
		t.Errorf("Expected count 1 after age-based eviction, got %d", store.Count())
	}

	recent := store.GetRecent(10)
	if len(recent) != 1 || recent[0].Message != "New message" {
		t.Error("Expected only new message to remain")
	}
}

func TestSearchByContainer(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	// Add messages for different containers
	for i := 0; i < 5; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     fmt.Sprintf("Container1 message %d", i),
			Fields:      map[string]string{},
		})
	}

	for i := 0; i < 3; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container2",
			Message:     fmt.Sprintf("Container2 message %d", i),
			Fields:      map[string]string{},
		})
	}

	// Search for container1
	results := store.SearchByContainer("container1", 10)
	if len(results) != 5 {
		t.Errorf("Expected 5 results for container1, got %d", len(results))
	}

	// Search for container2
	results = store.SearchByContainer("container2", 10)
	if len(results) != 3 {
		t.Errorf("Expected 3 results for container2, got %d", len(results))
	}

	// Search for non-existent container
	results = store.SearchByContainer("container3", 10)
	if results != nil {
		t.Errorf("Expected nil for non-existent container, got %d results", len(results))
	}
}

func TestSearchByField(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	// Add messages with different field values
	for i := 0; i < 3; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     fmt.Sprintf("Message with req1: %d", i),
			Fields:      map[string]string{"request_id": "req1"},
		})
	}

	for i := 0; i < 2; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     fmt.Sprintf("Message with req2: %d", i),
			Fields:      map[string]string{"request_id": "req2"},
		})
	}

	// Search for request_id=req1
	results := store.SearchByField("request_id", "req1", 10)
	if len(results) != 3 {
		t.Errorf("Expected 3 results for req1, got %d", len(results))
	}

	// Search for request_id=req2
	results = store.SearchByField("request_id", "req2", 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results for req2, got %d", len(results))
	}

	// Search for non-existent field
	results = store.SearchByField("trace_id", "trace1", 10)
	if results != nil {
		t.Errorf("Expected nil for non-existent field, got %d results", len(results))
	}
}

func TestSearchByText(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	store.Add(&LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Error processing request",
		Fields:      map[string]string{},
	})

	store.Add(&LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Successfully processed request",
		Fields:      map[string]string{},
	})

	store.Add(&LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Starting server",
		Fields:      map[string]string{},
	})

	// Search for "request"
	results := store.SearchByText("request", 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results containing 'request', got %d", len(results))
	}

	// Search for "Error"
	results = store.SearchByText("Error", 10)
	if len(results) != 1 {
		t.Errorf("Expected 1 result containing 'Error', got %d", len(results))
	}

	// Search for non-existent text
	results = store.SearchByText("nonexistent", 10)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for 'nonexistent', got %d", len(results))
	}
}

func TestSearchByFields(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	// Add messages with multiple fields
	store.Add(&LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Message 1",
		Fields: map[string]string{
			"request_id": "req1",
			"user_id":    "user1",
			"action":     "create",
		},
	})

	store.Add(&LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Message 2",
		Fields: map[string]string{
			"request_id": "req1",
			"user_id":    "user2",
			"action":     "create",
		},
	})

	store.Add(&LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "container1",
		Message:     "Message 3",
		Fields: map[string]string{
			"request_id": "req2",
			"user_id":    "user1",
			"action":     "delete",
		},
	})

	// Search for request_id=req1 AND user_id=user1
	filters := []FieldFilter{
		{Name: "request_id", Value: "req1"},
		{Name: "user_id", Value: "user1"},
	}
	results := store.SearchByFields(filters, 10)
	if len(results) != 1 {
		t.Errorf("Expected 1 result matching both filters, got %d", len(results))
	}
	if len(results) > 0 && results[0].Message != "Message 1" {
		t.Errorf("Expected 'Message 1', got %s", results[0].Message)
	}

	// Search for action=create
	filters = []FieldFilter{
		{Name: "action", Value: "create"},
	}
	results = store.SearchByFields(filters, 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results with action=create, got %d", len(results))
	}

	// Search for non-matching combination
	filters = []FieldFilter{
		{Name: "request_id", Value: "req1"},
		{Name: "action", Value: "delete"},
	}
	results = store.SearchByFields(filters, 10)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for non-matching filters, got %d results", len(results))
	}
}

func TestSearchComplex(t *testing.T) {
	store := NewLogStore(100, 2*time.Hour)

	now := time.Now()
	past := now.Add(-1 * time.Hour)

	// Add various messages
	store.Add(&LogMessage{
		Timestamp:   past,
		ContainerID: "container1",
		Message:     "Old message with error",
		Fields: map[string]string{
			"request_id": "req1",
			"level":      "error",
		},
	})

	store.Add(&LogMessage{
		Timestamp:   now,
		ContainerID: "container1",
		Message:     "Recent message with error",
		Fields: map[string]string{
			"request_id": "req2",
			"level":      "error",
		},
	})

	store.Add(&LogMessage{
		Timestamp:   now,
		ContainerID: "container2",
		Message:     "Recent message with info",
		Fields: map[string]string{
			"request_id": "req3",
			"level":      "info",
		},
	})

	// Search for container1 + level=error + containing "error"
	afterTime := past.Add(-1 * time.Minute)
	criteria := SearchCriteria{
		ContainerID: "container1",
		Fields: []FieldFilter{
			{Name: "level", Value: "error"},
		},
		TextSearch: "error",
		After:      &afterTime,
	}
	results := store.SearchComplex(criteria, 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results matching complex criteria, got %d", len(results))
	}

	// Search with Before filter
	beforeTime := now.Add(-30 * time.Minute)
	criteria = SearchCriteria{
		Before: &beforeTime,
	}
	results = store.SearchComplex(criteria, 10)
	if len(results) != 1 {
		t.Errorf("Expected 1 old message, got %d", len(results))
	}

	// Search for container2
	criteria = SearchCriteria{
		ContainerID: "container2",
	}
	results = store.SearchComplex(criteria, 10)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for container2, got %d", len(results))
	}
}

func TestSetMaxMessages(t *testing.T) {
	store := NewLogStore(10, 1*time.Hour)

	// Add 10 messages
	for i := 0; i < 10; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     fmt.Sprintf("Message %d", i),
			Fields:      map[string]string{},
		})
	}

	if store.Count() != 10 {
		t.Errorf("Expected count 10, got %d", store.Count())
	}

	// Reduce max to 5
	store.SetMaxMessages(5)

	if store.Count() != 5 {
		t.Errorf("Expected count 5 after SetMaxMessages, got %d", store.Count())
	}

	// Most recent messages should remain
	recent := store.GetRecent(10)
	if len(recent) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(recent))
	}
	if recent[0].Message != "Message 9" {
		t.Errorf("Expected most recent message 'Message 9', got %s", recent[0].Message)
	}
}

func TestIndexConsistency(t *testing.T) {
	// maxMessages is per-container, so with limit=3 and 2 containers, we can have up to 6 messages
	store := NewLogStore(3, 1*time.Hour)

	// Add 10 messages across 2 containers (5 per container)
	// With per-container limit of 3, each container will keep only the last 3
	for i := 0; i < 10; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: fmt.Sprintf("container%d", i%2),
			Message:     fmt.Sprintf("Message %d", i),
			Fields: map[string]string{
				"request_id": fmt.Sprintf("req%d", i%3),
			},
		})
	}

	// With 2 containers and limit 3 per container, total should be 6
	// container0: messages 4, 6, 8 (last 3)
	// container1: messages 5, 7, 9 (last 3)
	if store.Count() != 6 {
		t.Errorf("Expected count 6, got %d", store.Count())
	}

	// Verify container index - each container should have 3 messages
	container0Results := store.SearchByContainer("container0", 10)
	container1Results := store.SearchByContainer("container1", 10)

	if len(container0Results) != 3 {
		t.Errorf("Expected 3 results for container0, got %d", len(container0Results))
	}
	if len(container1Results) != 3 {
		t.Errorf("Expected 3 results for container1, got %d", len(container1Results))
	}

	totalContainerResults := len(container0Results) + len(container1Results)
	if totalContainerResults != 6 {
		t.Errorf("Expected 6 total container results, got %d", totalContainerResults)
	}

	// Verify field index is consistent with actual stored messages
	req0Results := store.SearchByField("request_id", "req0", 10)
	req1Results := store.SearchByField("request_id", "req1", 10)
	req2Results := store.SearchByField("request_id", "req2", 10)

	totalFieldResults := 0
	if req0Results != nil {
		totalFieldResults += len(req0Results)
	}
	if req1Results != nil {
		totalFieldResults += len(req1Results)
	}
	if req2Results != nil {
		totalFieldResults += len(req2Results)
	}

	// Total field results should match total message count
	if totalFieldResults != 6 {
		t.Errorf("Expected 6 total field results, got %d", totalFieldResults)
	}
}

func TestConcurrentAccess(t *testing.T) {
	store := NewLogStore(1000, 1*time.Hour)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				store.Add(&LogMessage{
					Timestamp:   time.Now(),
					ContainerID: fmt.Sprintf("container%d", id),
					Message:     fmt.Sprintf("Writer %d message %d", id, j),
					Fields: map[string]string{
						"writer": fmt.Sprintf("%d", id),
					},
				})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				store.GetRecent(10)
				store.SearchByContainer(fmt.Sprintf("container%d", id), 10)
				store.SearchByField("writer", fmt.Sprintf("%d", id), 10)
				store.Count()
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	if store.Count() != 1000 {
		t.Errorf("Expected count 1000 after concurrent access, got %d", store.Count())
	}
}

func TestEmptySearches(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	// Test searches on empty store
	if results := store.GetRecent(10); len(results) != 0 {
		t.Errorf("Expected 0 results from empty store, got %d", len(results))
	}

	if results := store.SearchByContainer("container1", 10); results != nil {
		t.Errorf("Expected nil from SearchByContainer on empty store, got %d results", len(results))
	}

	if results := store.SearchByField("field", "value", 10); results != nil {
		t.Errorf("Expected nil from SearchByField on empty store, got %d results", len(results))
	}

	if results := store.SearchByText("text", 10); len(results) != 0 {
		t.Errorf("Expected 0 results from SearchByText on empty store, got %d", len(results))
	}

	filters := []FieldFilter{{Name: "field", Value: "value"}}
	if results := store.SearchByFields(filters, 10); results != nil {
		t.Errorf("Expected nil from SearchByFields on empty store, got %d results", len(results))
	}

	criteria := SearchCriteria{ContainerID: "container1"}
	if results := store.SearchComplex(criteria, 10); results != nil {
		t.Errorf("Expected nil from SearchComplex on empty store, got %d results", len(results))
	}
}

func TestLimitParameter(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	// Add 20 messages
	for i := 0; i < 20; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     fmt.Sprintf("Message %d", i),
			Fields:      map[string]string{"field": "value"},
		})
	}

	// Test limit with GetRecent
	results := store.GetRecent(5)
	if len(results) != 5 {
		t.Errorf("Expected 5 results with limit, got %d", len(results))
	}

	// Test limit with SearchByContainer
	results = store.SearchByContainer("container1", 7)
	if len(results) != 7 {
		t.Errorf("Expected 7 results with limit, got %d", len(results))
	}

	// Test limit with SearchByField
	results = store.SearchByField("field", "value", 3)
	if len(results) != 3 {
		t.Errorf("Expected 3 results with limit, got %d", len(results))
	}
}

func TestFilter(t *testing.T) {
	store := NewLogStore(100, 1*time.Hour)

	// Add test messages with various attributes
	messages := []*LogMessage{
		{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     "Error in authentication",
			Fields: map[string]string{
				"_level":     "ERR",
				"request_id": "req1",
				"user_id":    "user123",
			},
		},
		{
			Timestamp:   time.Now(),
			ContainerID: "container1",
			Message:     "Debug message for user",
			Fields: map[string]string{
				"_level":     "DBG",
				"request_id": "req2",
				"user_id":    "user123",
			},
		},
		{
			Timestamp:   time.Now(),
			ContainerID: "container2",
			Message:     "Info: Processing request",
			Fields: map[string]string{
				"_level":     "INF",
				"request_id": "req1",
				"service":    "api",
			},
		},
		{
			Timestamp:   time.Now(),
			ContainerID: "container2",
			Message:     "Warning: High memory usage",
			Fields: map[string]string{
				"_level":  "WRN",
				"service": "api",
			},
		},
		{
			Timestamp:   time.Now(),
			ContainerID: "container3",
			Message:     "No level message",
			Fields: map[string]string{
				"request_id": "req3",
			},
		},
	}

	for _, msg := range messages {
		store.Add(msg)
	}

	// Test 1: Filter by container only
	opts := FilterOptions{
		ContainerIDs: []string{"container1"},
	}
	results := store.Filter(opts, 100)
	if len(results) != 2 {
		t.Errorf("Expected 2 results for container1, got %d", len(results))
	}

	// Test 2: Filter by level
	opts = FilterOptions{
		Levels: []string{"ERR", "WRN"},
	}
	results = store.Filter(opts, 100)
	if len(results) != 2 {
		t.Errorf("Expected 2 results for ERR/WRN levels, got %d", len(results))
	}

	// Test 3: Filter by search term
	opts = FilterOptions{
		SearchTerms: []string{"user"},
	}
	results = store.Filter(opts, 100)
	if len(results) != 2 {
		t.Errorf("Expected 2 results containing 'user', got %d", len(results))
	}

	// Test 4: Filter by field
	opts = FilterOptions{
		FieldFilters: []FieldFilter{
			{Name: "request_id", Value: "req1"},
		},
	}
	results = store.Filter(opts, 100)
	if len(results) != 2 {
		t.Errorf("Expected 2 results with request_id=req1, got %d", len(results))
	}

	// Test 5: Combined filters (container + level)
	opts = FilterOptions{
		ContainerIDs: []string{"container1"},
		Levels:       []string{"ERR"},
	}
	results = store.Filter(opts, 100)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for container1 + ERR, got %d", len(results))
	}
	if len(results) > 0 && results[0].Message != "Error in authentication" {
		t.Errorf("Expected 'Error in authentication', got %s", results[0].Message)
	}

	// Test 6: Multiple search terms (AND logic)
	opts = FilterOptions{
		SearchTerms: []string{"user", "debug"},
	}
	results = store.Filter(opts, 100)
	if len(results) != 1 {
		t.Errorf("Expected 1 result matching both 'user' AND 'debug', got %d", len(results))
	}

	// Test 7: Filter by NONE level
	opts = FilterOptions{
		Levels: []string{"NONE"},
	}
	results = store.Filter(opts, 100)
	if len(results) != 1 {
		t.Errorf("Expected 1 result with no level (NONE), got %d", len(results))
	}

	// Test 8: Filter with limit
	opts = FilterOptions{}
	results = store.Filter(opts, 3)
	if len(results) != 3 {
		t.Errorf("Expected 3 results with limit, got %d", len(results))
	}

	// Test 9: No matching results
	opts = FilterOptions{
		ContainerIDs: []string{"nonexistent"},
	}
	results = store.Filter(opts, 100)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for nonexistent container, got %d", len(results))
	}

	// Test 10: Complex filter - multiple containers + level + field
	opts = FilterOptions{
		ContainerIDs: []string{"container1", "container2"},
		Levels:       []string{"ERR", "INF"},
		FieldFilters: []FieldFilter{
			{Name: "request_id", Value: "req1"},
		},
	}
	results = store.Filter(opts, 100)
	if len(results) != 2 {
		t.Errorf("Expected 2 results for complex filter, got %d", len(results))
	}
}

func TestContainerRetentionByTime(t *testing.T) {
	store := NewLogStore(1000, 1*time.Hour)

	containerID := "test-container"
	now := time.Now()

	// Add 120 old messages (older than 5 seconds) to exceed the minimum of 100
	for i := 0; i < 120; i++ {
		store.Add(&LogMessage{
			Timestamp:   now.Add(-10 * time.Second),
			ContainerID: containerID,
			Message:     fmt.Sprintf("Old message %d", i),
			Fields:      map[string]string{},
		})
	}

	// Add 5 recent messages (within last second)
	for i := 0; i < 5; i++ {
		store.Add(&LogMessage{
			Timestamp:   now.Add(-1 * time.Second),
			ContainerID: containerID,
			Message:     fmt.Sprintf("Recent message %d", i),
			Fields:      map[string]string{},
		})
	}

	// Verify we have 125 messages total
	if store.CountByContainer(containerID) != 125 {
		t.Errorf("Expected 125 messages before retention, got %d", store.CountByContainer(containerID))
	}

	// Set time-based retention policy: keep only logs from last 5 seconds
	policy := ContainerRetentionPolicy{
		Type:  "time",
		Value: 5, // 5 seconds
	}
	store.SetContainerRetention(containerID, policy)

	// After applying retention, should keep at least 100 messages (the minimum)
	// Since we have 5 recent + 120 old, and we want to keep only recent (5),
	// but must keep at least 100, we should have exactly 100 messages
	count := store.CountByContainer(containerID)
	if count != 100 {
		t.Errorf("Expected 100 messages after time-based retention (minimum kept), got %d", count)
	}

	// Verify we removed 25 old messages (125 - 100 = 25)
	// The remaining 100 should be: 5 recent + 95 oldest of the old messages
}

func TestContainerRetentionByCount(t *testing.T) {
	store := NewLogStore(1000, 1*time.Hour)

	containerID := "test-container"

	// Add 20 messages
	for i := 0; i < 20; i++ {
		store.Add(&LogMessage{
			Timestamp:   time.Now(),
			ContainerID: containerID,
			Message:     fmt.Sprintf("Message %d", i),
			Fields:      map[string]string{},
		})
	}

	// Verify we have 20 messages
	if store.CountByContainer(containerID) != 20 {
		t.Errorf("Expected 20 messages before retention, got %d", store.CountByContainer(containerID))
	}

	// Set count-based retention policy: keep only 10 messages
	policy := ContainerRetentionPolicy{
		Type:  "count",
		Value: 10,
	}
	store.SetContainerRetention(containerID, policy)

	// After applying retention, should have only 10 messages
	count := store.CountByContainer(containerID)
	if count != 10 {
		t.Errorf("Expected 10 messages after count-based retention, got %d", count)
	}

	// Verify the remaining messages are the most recent ones (19 down to 10)
	results := store.SearchByContainer(containerID, 100)
	if len(results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(results))
	}
	// Most recent should be "Message 19"
	if results[0].Message != "Message 19" {
		t.Errorf("Expected most recent message 'Message 19', got %s", results[0].Message)
	}
	// Oldest should be "Message 10"
	if results[9].Message != "Message 10" {
		t.Errorf("Expected oldest remaining message 'Message 10', got %s", results[9].Message)
	}
}

func TestContainerRetentionMinKeep(t *testing.T) {
	store := NewLogStore(1000, 1*time.Hour)

	containerID := "test-container"
	now := time.Now()

	// Add 150 old messages (all older than cutoff)
	for i := 0; i < 150; i++ {
		store.Add(&LogMessage{
			Timestamp:   now.Add(-20 * time.Second),
			ContainerID: containerID,
			Message:     fmt.Sprintf("Old message %d", i),
			Fields:      map[string]string{},
		})
	}

	// Set time-based retention with very short time
	policy := ContainerRetentionPolicy{
		Type:  "time",
		Value: 1, // 1 second - all messages are older
	}
	store.SetContainerRetention(containerID, policy)

	// Should keep at least 100 messages even though all are older than cutoff
	count := store.CountByContainer(containerID)
	if count != 100 {
		t.Errorf("Expected 100 messages kept (minimum), got %d", count)
	}
}
