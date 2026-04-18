package e2e

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketEcho(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()

		// Echo received messages
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(mt, msg)
		}
	}))
	defer server.Close()

	// Connect client
	wsURL := "ws" + server.URL[4:]
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer ws.Close()

	// Send message
	testMsg := []byte("hello")
	if err := ws.WriteMessage(websocket.TextMessage, testMsg); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Receive echo
	_, recvMsg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(recvMsg) != string(testMsg) {
		t.Errorf("Echo mismatch: got %s, want %s", string(recvMsg), string(testMsg))
	}
}

func TestWebSocketMultipleClients(t *testing.T) {
	// Create test server with broadcast
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()

		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			conn.WriteMessage(mt, msg)
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]

	// Connect 3 clients
	var clients []*websocket.Conn
	for i := 0; i < 3; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Dial %d failed: %v", i, err)
		}
		clients = append(clients, ws)
		defer ws.Close()
	}

	// Test concurrent sends
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := []byte{byte('A' + idx)}
			clients[idx].WriteMessage(websocket.TextMessage, msg)
		}(i)
	}
	wg.Wait()

	// Give time for processing
	time.Sleep(100 * time.Millisecond)
}

func TestWebSocketDisconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)

		// Wait for close
		conn.ReadMessage()
		conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	// Close from client side
	ws.Close()

	// Server should handle gracefully
	time.Sleep(100 * time.Millisecond)
}