package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func TestRoomHubSerializesHydrationAndConcurrentBroadcasts(t *testing.T) {
	hub := NewRoomHub()
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		conn, err := liveUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		writer := hub.add("room", conn)
		defer hub.remove("room", conn)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					hub.Broadcast("room", []byte{3, 1, 2, 3})
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if err := writer.write([]byte{5, 4, 5, 6}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
		wg.Wait()
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for i := 0; i < 450; i++ {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) != 4 || (payload[0] != 3 && payload[0] != 5) {
			t.Fatalf("corrupt frame: %v", payload)
		}
	}
	<-finished
}
