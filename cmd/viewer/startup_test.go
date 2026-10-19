package main

import (
	"os"
	"strings"
	"testing"
)

func TestNewWebAppRejectsUnavailableStore(t *testing.T) {
	t.Chdir(t.TempDir())
	// A directory at the database path forces SQLite initialization to fail.
	if err := os.Mkdir("graphql-requests.db", 0700); err != nil {
		t.Fatal(err)
	}

	app, err := NewWebApp()
	if app != nil {
		app.cancel()
		app.docker.Close()
		if app.store != nil {
			app.store.Close()
		}
		t.Fatal("expected no application when the store cannot be opened")
	}
	if err == nil || !strings.Contains(err.Error(), "failed to initialize store") {
		t.Fatalf("expected store initialization error, got %v", err)
	}
}
