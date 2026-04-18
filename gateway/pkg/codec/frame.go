package codec

import (
    "encoding/binary"
    "errors"
)

const (
    FrameTypeHeartbeat      = 0x01
    FrameTypeTerminalOutput = 0x02
    FrameTypeTerminalInput  = 0x03
    FrameTypeWindowResize   = 0x04
    FrameTypeSessionControl = 0x05
)

var (
    ErrBufferTooSmall = errors.New("buffer too small")
    ErrInvalidFrame   = errors.New("invalid frame")
)

type Frame struct {
    Type         uint8
    SessionCount uint8
    Sessions     []SessionBlock
}

type SessionBlock struct {
    SessionID uint32
    Length    uint16
    Data      []byte
}

func (f *Frame) Serialize() []byte {
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

func Deserialize(data []byte) (*Frame, error) {
    if len(data) < 2 {
        return nil, ErrBufferTooSmall
    }
    frameType := data[0]
    sessionCount := data[1]
    offset := 2
    sessions := make([]SessionBlock, 0, sessionCount)
    for i := uint8(0); i < sessionCount; i++ {
        if offset+6 > len(data) {
            return nil, ErrBufferTooSmall
        }
        sessionID := binary.BigEndian.Uint32(data[offset:])
        offset += 4
        length := binary.BigEndian.Uint16(data[offset:])
        offset += 2
        if offset+int(length) > len(data) {
            return nil, ErrBufferTooSmall
        }
        session := SessionBlock{
            SessionID: sessionID,
            Length:    length,
            Data:      make([]byte, length),
        }
        copy(session.Data, data[offset:offset+int(length)])
        offset += int(length)
        sessions = append(sessions, session)
    }
    return &Frame{
        Type:         frameType,
        SessionCount: sessionCount,
        Sessions:     sessions,
    }, nil
}