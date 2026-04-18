import { describe, it, expect } from 'vitest';
import { parseFrame, FrameType } from './parser';

describe('parseFrame', () => {
  it('should parse a valid frame', () => {
    // Create a test frame: Type=0x02, SessionCount=1, SessionID=12345, Length=5, Data="hello"
    // Total: 2 (header) + 6 (session header) + 5 (data) = 13 bytes
    const buf = new ArrayBuffer(13);
    const view = new DataView(buf);
    view.setUint8(0, 0x02); // Type
    view.setUint8(1, 1);     // SessionCount
    view.setUint32(2, 12345, false); // SessionID (BigEndian)
    view.setUint16(6, 5, false);     // Length (BigEndian)
    const text = new TextEncoder().encode('hello');
    new Uint8Array(buf, 8, 5).set(text);

    const result = parseFrame(buf);
    expect(result).not.toBeNull();
    expect(result!.type).toBe(0x02);
    expect(result!.sessionCount).toBe(1);
    expect(result!.sessions[0].sessionId).toBe(12345);
    expect(result!.sessions[0].length).toBe(5);
  });

  it('should return null for buffer too small', () => {
    const buf = new ArrayBuffer(1);
    const result = parseFrame(buf);
    expect(result).toBeNull();
  });

  it('should parse multiple sessions', () => {
    // Frame with 2 sessions
    // Session 1: ID=1, Length=3, Data="abc"
    // Session 2: ID=2, Length=4, Data="defg"
    const session1Data = new TextEncoder().encode('abc');
    const session2Data = new TextEncoder().encode('defg');
    const buf = new ArrayBuffer(2 + 6 + 3 + 6 + 4); // header + session1 + session2
    const view = new DataView(buf);

    view.setUint8(0, FrameType.TerminalOutput);
    view.setUint8(1, 2); // 2 sessions

    // Session 1
    view.setUint32(2, 1, false); // SessionID
    view.setUint16(6, 3, false); // Length
    new Uint8Array(buf, 8, 3).set(session1Data);

    // Session 2
    view.setUint32(11, 2, false); // SessionID
    view.setUint16(15, 4, false); // Length
    new Uint8Array(buf, 17, 4).set(session2Data);

    const result = parseFrame(buf);
    expect(result).not.toBeNull();
    expect(result!.type).toBe(FrameType.TerminalOutput);
    expect(result!.sessionCount).toBe(2);
    expect(result!.sessions.length).toBe(2);
    expect(result!.sessions[0].sessionId).toBe(1);
    expect(result!.sessions[0].data).toEqual(session1Data);
    expect(result!.sessions[1].sessionId).toBe(2);
    expect(result!.sessions[1].data).toEqual(session2Data);
  });

  it('should return null for incomplete session header', () => {
    // Buffer has header but not enough for full session header (need 8 bytes total: 2 header + 6 session)
    const buf = new ArrayBuffer(5);
    const view = new DataView(buf);
    view.setUint8(0, FrameType.TerminalOutput);
    view.setUint8(1, 1); // 1 session
    // Session header needs 6 bytes but only 3 bytes remain (offsets 2,3,4)

    const result = parseFrame(buf);
    expect(result).toBeNull();
  });
});
