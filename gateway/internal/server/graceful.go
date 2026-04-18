package server

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

type GracefulState struct {
	Sessions []SessionMeta `json:"sessions"`
	Version  string        `json:"version"`
}

type SessionMeta struct {
	SessionID uint32 `json:"sessionId"`
	UserID    string `json:"userId"`
	HostID    string `json:"hostId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

type GracefulManager struct {
	hub        *Hub
	state      *GracefulState
	listener   net.Listener
	socketPath string
	version    string
}

func NewGracefulManager(hub *Hub, socketPath string, version string) *GracefulManager {
	return &GracefulManager{
		hub:        hub,
		socketPath: socketPath,
		version:    version,
	}
}

// PrepareForGracefulShutdown prepares session metadata for serialization
func (gm *GracefulManager) PrepareForGracefulShutdown() (*GracefulState, error) {
	sessions := make([]SessionMeta, 0)

	gm.hub.ConnMap.Range(func(key, value any) bool {
		conn := value.(*Conn)
		sessions = append(sessions, SessionMeta{
			SessionID: conn.ID,
		})
		return true
	})

	return &GracefulState{
		Sessions: sessions,
		Version:  gm.version,
	}, nil
}

// BroadcastMaintenance sends maintenance mode message to all clients
func (gm *GracefulManager) BroadcastMaintenance(message string) {
	data := []byte(fmt.Sprintf(`{"type":"maintenance","message":"%s"}`, message))

	frame := &MaintenanceFrame{
		Type:         0x07, // Maintenance frame type
		SessionCount: 1,
		Sessions: []MaintenanceBlock{
			{
				SessionID: 0, // Broadcast to all
				Length:    uint16(len(data)),
				Data:      data,
			},
		},
	}

	gm.hub.Broadcast(frame.Serialize())
}

// HandleUSR2Signal handles SIGUSR2 for graceful restart
func (gm *GracefulManager) HandleUSR2Signal() {
	log.Println("Received SIGUSR2, starting graceful restart...")

	// 1. Broadcast maintenance message
	gm.BroadcastMaintenance("Server is restarting, please wait...")

	// 2. Prepare state for serialization
	state, err := gm.PrepareForGracefulShutdown()
	if err != nil {
		log.Printf("Failed to prepare state: %v", err)
		return
	}
	gm.state = state

	// 3. Serialize state to Redis (simplified to file for now)
	stateJSON, err := json.Marshal(state)
	if err != nil {
		log.Printf("Failed to marshal state: %v", err)
		return
	}
	if err := os.WriteFile("/tmp/gateway_state.json", stateJSON, 0644); err != nil {
		log.Printf("Failed to write state file: %v", err)
		return
	}

	// 4. Create Unix Domain Socket for FD handoff
	if gm.socketPath != "" {
		gm.startHandoffListener()
	}

	log.Println("Graceful restart preparation complete, waiting for new process to take over")
}

// startHandoffListener creates a Unix Domain Socket for FD handoff
func (gm *GracefulManager) startHandoffListener() {
	if gm.listener != nil {
		gm.listener.Close()
	}

	// Remove existing socket
	os.Remove(gm.socketPath)

	listener, err := net.Listen("unix", gm.socketPath)
	if err != nil {
		log.Printf("Failed to create handoff socket: %v", err)
		return
	}
	gm.listener = listener

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			return
		}
		defer conn.Close()

		// Send state via socket
		stateJSON, _ := json.Marshal(gm.state)
		conn.Write(stateJSON)

		// Signal parent to exit
		log.Println("FD handoff complete, preparing to exit")
	}()
}

// WaitForHandoff waits for new process to take over
func (gm *GracefulManager) WaitForHandoff(timeout time.Duration) error {
	if gm.listener == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		conn, err := gm.listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		done <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// MaintenanceFrame is used for graceful restart maintenance broadcasts
type MaintenanceFrame struct {
	Type         uint8
	SessionCount uint8
	Sessions     []MaintenanceBlock
}

type MaintenanceBlock struct {
	SessionID uint32
	Length    uint16
	Data      []byte
}

func (f *MaintenanceFrame) Serialize() []byte {
	totalLen := 2 // FrameType + SessionCount
	for _, s := range f.Sessions {
		totalLen += 4 + 2 + len(s.Data)
	}
	buf := make([]byte, totalLen)
	buf[0] = f.Type
	buf[1] = f.SessionCount
	offset := 2
	for _, s := range f.Sessions {
		binary.BigEndian.PutUint32(buf[offset:], s.SessionID)
		offset += 4
		binary.BigEndian.PutUint16(buf[offset:], s.Length)
		offset += 2
		copy(buf[offset:], s.Data)
		offset += int(s.Length)
	}
	return buf
}