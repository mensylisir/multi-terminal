package session

import (
    "sync"
    "time"
)

const (
    DefaultTTL        = 10 * time.Minute
    CleanupInterval   = 1 * time.Minute
)

type Manager struct {
    sessions     sync.Map
    ttl          time.Duration
    cleanupTick  *time.Ticker
    stopCleanup  chan struct{}
}

func NewManager() *Manager {
    m := &Manager{
        ttl:        DefaultTTL,
        stopCleanup: make(chan struct{}),
    }
    go m.cleanupLoop()
    return m
}

func (m *Manager) cleanupLoop() {
    m.cleanupTick = time.NewTicker(CleanupInterval)
    for {
        select {
        case <-m.cleanupTick.C:
            m.cleanup()
        case <-m.stopCleanup:
            m.cleanupTick.Stop()
            return
        }
    }
}

func (m *Manager) cleanup() {
    m.sessions.Range(func(key, value any) bool {
        s := value.(*Session)
        if s.GetState() == StateDetached && time.Since(s.LastActiveAt) > m.ttl {
            m.sessions.Delete(key)
        }
        return true
    })
}

func (m *Manager) Register(s *Session) {
    m.sessions.Store(s.SessionID, s)
}

func (m *Manager) Unregister(id uint32) {
    m.sessions.Delete(id)
}

func (m *Manager) Get(id uint32) (*Session, bool) {
    v, ok := m.sessions.Load(id)
    if !ok {
        return nil, false
    }
    return v.(*Session), true
}

func (m *Manager) Range(f func(*Session) bool) {
    m.sessions.Range(func(key, value any) bool {
        return f(value.(*Session))
    })
}

func (m *Manager) Stop() {
    close(m.stopCleanup)
}