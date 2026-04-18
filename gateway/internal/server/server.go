package server

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	IDGen       uint32
	ConnMap     sync.Map
	Register    chan *Conn
	Unregister  chan *Conn
}

func NewHub() *Hub {
	return &Hub{
		Register:   make(chan *Conn),
		Unregister: make(chan *Conn),
	}
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

func HandleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn := NewConn(0, ws)
	HubGlobal.Register <- conn
	go conn.WritePump()
	go conn.ReadPump(func(msg []byte) {
		// Echo back for testing
		conn.Send <- msg
	})
}