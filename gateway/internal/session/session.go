package session

import (
    "io"
    "sync"
    "time"

    "golang.org/x/crypto/ssh"
)

type PromptState int

const (
    PromptShell   PromptState = iota
    PromptTUI
    PromptUnknown
)

type Session struct {
    SessionID       uint32
    UserID          string
    HostID          string
    SSHClient       *ssh.Client
    ShellStream     io.ReadWriteCloser
    Cols            int
    Rows            int
    State           State
    LastActiveAt    time.Time
    WriteBufferSize int
    IsSlow          bool
    PromptState     PromptState
    mu              sync.RWMutex
}

func NewSession(id uint32, userID, hostID string) *Session {
    return &Session{
        SessionID:    id,
        UserID:       userID,
        HostID:       hostID,
        Cols:         80,
        Rows:         24,
        State:        StateActive,
        LastActiveAt: time.Now(),
        PromptState:  PromptUnknown,
    }
}

func (s *Session) SetState(state State) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.State = state
}

func (s *Session) GetState() State {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.State
}

func (s *Session) SetPromptState(ps PromptState) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.PromptState = ps
}

func (s *Session) GetPromptState() PromptState {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.PromptState
}

func (s *Session) SetSlow(slow bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.IsSlow = slow
}

func (s *Session) GetSlow() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.IsSlow
}