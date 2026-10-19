package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"

	"github.com/gorilla/websocket"
)

func TestConcurrentWebSocketMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := logstore.NewLogStore(100, time.Hour)
	msg := logs.ContainerMessage{
		ContainerID: "test-container",
		Timestamp:   time.Now(),
		Entry:       &logs.LogEntry{Message: strings.Repeat("log message ", 24000)},
	}
	store.Add(&msg)
	ctrl := NewController(nil, store, nil, ctx, cancel, nil)
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		conn, err := ctrl.upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		client := &Client{conn: conn}
		ctrl.clients[client] = true
		var writers sync.WaitGroup
		start := make(chan struct{})
		for _, send := range []func(){
			func() { ctrl.sendInitialLogs(client) },
			func() { ctrl.BroadcastBatch([]logs.ContainerMessage{msg}) },
			func() { ctrl.BroadcastContainerUpdate(nil) },
		} {
			writers.Add(1)
			go func() {
				defer writers.Done()
				defer func() {
					if p := recover(); p != nil {
						t.Error(fmt.Sprint(p))
						conn.Close()
					}
				}()
				<-start
				for range 20 {
					send()
				}
			}()
		}
		close(start)
		writers.Wait()
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for range 60 {
		var message WSMessage
		if err := conn.ReadJSON(&message); err != nil {
			t.Errorf("expected complete JSON WebSocket message: %v", err)
			break
		}
	}
	<-done
}
