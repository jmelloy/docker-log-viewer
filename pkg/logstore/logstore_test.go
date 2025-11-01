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
	store := NewLogStore(5, 1*time.Hour)

	// Add messages that will be evicted
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

	// Verify count
	if store.Count() != 5 {
		t.Errorf("Expected count 5, got %d", store.Count())
	}

	// Verify container index
	container0Results := store.SearchByContainer("container0", 10)
	container1Results := store.SearchByContainer("container1", 10)

	totalContainerResults := 0
	if container0Results != nil {
		totalContainerResults += len(container0Results)
	}
	if container1Results != nil {
		totalContainerResults += len(container1Results)
	}

	if totalContainerResults != 5 {
		t.Errorf("Expected 5 total container results, got %d", totalContainerResults)
	}

	// Verify field index
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

	if totalFieldResults != 5 {
		t.Errorf("Expected 5 total field results, got %d", totalFieldResults)
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
