package codec

import (
    "testing"
)

func TestFrameSerializeDeserialize(t *testing.T) {
    frame := &Frame{
        Type:         FrameTypeTerminalOutput,
        SessionCount: 1,
        Sessions: []SessionBlock{
            {
                SessionID: 12345,
                Length:    5,
                Data:      []byte("hello"),
            },
        },
    }
    data := frame.Serialize()
    parsed, err := Deserialize(data)
    if err != nil {
        t.Fatalf("Deserialize failed: %v", err)
    }
    if parsed.Type != frame.Type {
        t.Errorf("Type mismatch: got %d, want %d", parsed.Type, frame.Type)
    }
    if parsed.SessionCount != frame.SessionCount {
        t.Errorf("SessionCount mismatch: got %d, want %d", parsed.SessionCount, frame.SessionCount)
    }
    if len(parsed.Sessions) != 1 {
        t.Fatalf("Sessions length: got %d, want 1", len(parsed.Sessions))
    }
    if parsed.Sessions[0].SessionID != 12345 {
        t.Errorf("SessionID mismatch")
    }
    if string(parsed.Sessions[0].Data) != "hello" {
        t.Errorf("Data mismatch: got %s", string(parsed.Sessions[0].Data))
    }
}