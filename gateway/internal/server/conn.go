package server

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pingPeriod = 30 * time.Second
	pongWait   = 60 * time.Second
	writeWait  = 10 * time.Second
)

type Conn struct {
	ID   uint32
	WS   *websocket.Conn
	Send chan []byte
	mu   sync.Mutex
}

func NewConn(id uint32, ws *websocket.Conn) *Conn {
	return &Conn{
		ID:   id,
		WS:   ws,
		Send: make(chan []byte, 256),
	}
}

func (c *Conn) ReadPump(handler func([]byte)) {
	defer func() {
		c.WS.Close()
		close(c.Send)
	}()
	c.WS.SetReadLimit(65535)
	c.WS.SetReadDeadline(time.Now().Add(pongWait))
	c.WS.SetPongHandler(func(string) error {
		c.WS.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, msg, err := c.WS.ReadMessage()
		if err != nil {
			break
		}
		handler(msg)
	}
}

func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.WS.Close()
		close(c.Send)
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			c.WS.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.WS.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.mu.Lock()
			err := c.WS.WriteMessage(websocket.BinaryMessage, msg)
			c.mu.Unlock()
			if err != nil {
				return
			}
		case <-ticker.C:
			c.WS.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.WS.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}