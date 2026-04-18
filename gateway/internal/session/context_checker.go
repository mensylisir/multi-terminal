package session

import (
    "fmt"
    "os"
    "regexp"
    "time"
)

var (
    shellPromptRegex = regexp.MustCompile(`[\$#]\s*$`)
    tuiEscapeRegex   = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
)

const (
    ProbeTimeout = 2 * time.Second
)

type ProbeResult struct {
    SessionID   uint32
    PromptState PromptState
    RawResponse []byte
}

// ProbeSession sends a probe to determine if the session is in shell, TUI, or unknown state
func (mgr *Manager) ProbeSession(sessionID uint32) (*ProbeResult, error) {
    s, ok := mgr.Get(sessionID)
    if !ok {
        return nil, fmt.Errorf("session not found")
    }

    // 发送探针 - 使用 ENQ (0x05)
    probe := []byte{0x05}
    _, err := s.ShellStream.Write(probe)
    if err != nil {
        return nil, err
    }

    // 等待响应
    buf := make([]byte, 1024)
    result := &ProbeResult{SessionID: sessionID}

    deadline := time.Now().Add(ProbeTimeout)
    for time.Now().Before(deadline) {
        s.ShellStream.SetReadDeadline(deadline)
        n, err := s.ShellStream.Read(buf)
        if err != nil {
            if os.IsTimeout(err) {
                result.PromptState = PromptUnknown
                return result, nil
            }
            if err.Error() == "EOF" {
                result.PromptState = PromptUnknown
                return result, nil
            }
            return nil, err
        }
        result.RawResponse = append(result.RawResponse, buf[:n]...)

        // 分析响应
        if shellPromptRegex.Match(result.RawResponse) {
            result.PromptState = PromptShell
            return result, nil
        }
        if tuiEscapeRegex.Match(result.RawResponse) {
            result.PromptState = PromptTUI
            return result, nil
        }
    }

    result.PromptState = PromptUnknown
    return result, nil
}