package router

import (
	"fmt"
	"sync"
	"time"

	"github.com/mensylisir/multi-terminal/gateway/pkg/buffer"
	"github.com/mensylisir/multi-terminal/gateway/pkg/codec"
	"github.com/mensylisir/multi-terminal/gateway/internal/server"
	"github.com/mensylisir/multi-terminal/gateway/internal/session"
)

const (
 AggregateInterval = 20 * time.Millisecond
)

type Router struct {
	buffers    sync.Map
	hub        *server.Hub
	sessionMgr *session.Manager
	ticker     *time.Ticker
	stop       chan struct{}
	pool       *buffer.Pool
}

func NewRouter(hub *server.Hub, sessionMgr *session.Manager, pool *buffer.Pool) *Router {
	return &Router{
		hub:        hub,
		sessionMgr: sessionMgr,
		pool:       pool,
		stop:       make(chan struct{}),
	}
}

func (r *Router) RegisterBuffer(sessionID uint32) *RingBuffer {
	rb := NewRingBuffer()
	r.buffers.Store(sessionID, rb)
	return rb
}

func (r *Router) UnregisterBuffer(sessionID uint32) {
	r.buffers.Delete(sessionID)
}

func (r *Router) Start() {
	r.ticker = time.NewTicker(AggregateInterval)
	go r.run()
}

func (r *Router) Stop() {
	r.ticker.Stop()
	close(r.stop)
}

func (r *Router) run() {
	for {
		select {
		case <-r.ticker.C:
			r.aggregate()
		case <-r.stop:
			return
		}
	}
}

func (r *Router) aggregate() {
	frame := &codec.Frame{
		Type:         codec.FrameTypeTerminalOutput,
		SessionCount: 0,
		Sessions:     make([]codec.SessionBlock, 0),
	}

	r.buffers.Range(func(key, value any) bool {
		rb := value.(*RingBuffer)
		sessionID := key.(uint32)

		s, ok := r.sessionMgr.Get(sessionID)
		if !ok {
			return true
		}

		// Backpressure: check water marks
		if rb.IsHigh() && !s.GetSlow() {
			s.SetSlow(true)
			r.sendSlowWarning(sessionID, true)
		} else if rb.IsLow() && s.GetSlow() {
			s.SetSlow(false)
			r.sendSlowWarning(sessionID, false)
		}

		// Skip slow nodes
		if s.GetSlow() {
			return true
		}

		// Get buffer from pool for reading
		data := r.pool.Get(4096)
		defer r.pool.Put(data)

		// Non-blocking read
		n, _ := rb.TryRead(data)
		if n > 0 {
			// Copy data since we'll return the buffer immediately
			sessionData := make([]byte, n)
			copy(sessionData, data[:n])

			frame.Sessions = append(frame.Sessions, codec.SessionBlock{
				SessionID: sessionID,
				Length:    uint16(n),
				Data:      sessionData,
			})
			frame.SessionCount++
		}
		return true
	})

	if frame.SessionCount > 0 {
		serialized := frame.Serialize()
		r.hub.Broadcast(serialized)
	}
}

func (r *Router) sendSlowWarning(sessionID uint32, isSlow bool) {
	data := []byte(fmt.Sprintf(`{"type":"slow","sessionId":%d,"isSlow":%v}`, sessionID, isSlow))

	frame := &codec.Frame{
		Type:         0x06, // SlowWarning frame type
		SessionCount: 1,
		Sessions: []codec.SessionBlock{
			{
				SessionID: sessionID,
				Length:    uint16(len(data)),
				Data:      data,
			},
		},
	}
	r.hub.Broadcast(frame.Serialize())
}