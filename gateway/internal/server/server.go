package server

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"

	"github.com/mensylisir/multi-terminal/gateway/internal/risk"
	"github.com/mensylisir/multi-terminal/gateway/internal/session"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	IDGen            uint32
	ConnMap          sync.Map
	Register         chan *Conn
	Unregister       chan *Conn
	SessionManager   *session.Manager
	ReconnectHandler *session.ReconnectHandler
}

func NewHub() *Hub {
	return &Hub{
		Register:   make(chan *Conn),
		Unregister: make(chan *Conn),
	}
}

// SetSessionManager sets the session manager and creates reconnect handler
func (h *Hub) SetSessionManager(mgr *session.Manager) {
	h.SessionManager = mgr
	h.ReconnectHandler = session.NewReconnectHandler(mgr)
}

func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.Register:
			atomic.StoreUint32(&conn.ID, atomic.AddUint32(&h.IDGen, 1))
			h.ConnMap.Store(conn.ID, conn)
		case conn := <-h.Unregister:
			h.ConnMap.Delete(conn.ID)
			close(conn.Send)
		}
	}
}

func (h *Hub) Broadcast(data []byte) {
	h.ConnMap.Range(func(key, value any) bool {
		conn := value.(*Conn)
		select {
		case conn.Send <- data:
		default:
		}
		return true
	})
}

var HubGlobal = NewHub()

var riskEngine = risk.NewEngine()

func init() {
	riskEngine.AddDefaultRules()
}

// handleInput handles terminal input with risk checking
func handleInput(sessionID uint32, data []byte) (bool, []byte) {
	command := string(data)

	// Check if blocked
	if blocked, rule := riskEngine.CheckAndBlock(command); blocked {
		msg := riskEngine.GetBlockMessage(rule)
		return false, []byte(msg)
	}

	// Check if confirmation required
	if confirm, rule := riskEngine.CheckAndConfirm(command); confirm {
		msg := riskEngine.GetConfirmMessage(rule)
		// Return confirmation required signal
		return true, []byte(msg)
	}

	return true, data
}

func HandleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Check for reconnection request via sessionId query parameter
	if sessionIDStr := r.URL.Query().Get("sessionId"); sessionIDStr != "" {
		var sessionID uint32
		if _, err := fmt.Sscanf(sessionIDStr, "%d", &sessionID); err == nil {
			// Handle reconnection if session manager is available
			if HubGlobal.SessionManager != nil && HubGlobal.ReconnectHandler != nil {
				s, err := HubGlobal.ReconnectHandler.HandleReconnect(sessionID)
				if err == nil && s != nil {
					// Reconnection successful
					// Note: Buffer replay would be handled by the router when session attaches
					// For now, we just mark the session as attached
					conn := NewConn(sessionID, ws)
					HubGlobal.Register <- conn
					go conn.WritePump()
					go conn.ReadPump(func(msg []byte) {
						// Handle incoming messages
						conn.Send <- msg
					})
					return
				}
			}
		}
	}

	// New connection
	conn := NewConn(0, ws)
	HubGlobal.Register <- conn
	go conn.WritePump()
	go conn.ReadPump(func(msg []byte) {
		// Handle input with risk checking
		allowed, output := handleInput(conn.ID, msg)
		if !allowed {
			// Command was blocked, send block message
			conn.Send <- output
			return
		}
		// For now, echo back for testing (or forward to session)
		conn.Send <- output
	})
}