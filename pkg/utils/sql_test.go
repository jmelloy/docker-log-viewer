package utils

import "testing"

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "replace parameter placeholders",
			query:    "SELECT * FROM users WHERE id = $1 AND name = $2",
			expected: "SELECT * FROM users WHERE id = ? AND name = ?",
		},
		{
			name:     "replace quoted strings",
			query:    "SELECT * FROM users WHERE name = 'John Doe'",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "replace numbers",
			query:    "SELECT * FROM users WHERE age > 18 AND id = 123",
			expected: "SELECT * FROM users WHERE age > ? AND id = ?",
		},
		{
			name:     "collapse whitespace",
			query:    "SELECT  *   FROM   users\n  WHERE   id = 1",
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:     "complex query",
			query:    "SELECT u.id, u.name FROM users u WHERE u.id IN ($1, $2, $3) AND u.age > 25",
			expected: "SELECT u.id, u.name FROM users u WHERE u.id IN (?, ?, ?) AND u.age > ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeQuery(tt.query)
			if result != tt.expected {
				t.Errorf("NormalizeQuery() = %q, want %q", result, tt.expected)
			}
		})
	}
}
