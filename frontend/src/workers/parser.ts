export const FrameType = {
  Heartbeat: 0x01,
  TerminalOutput: 0x02,
  TerminalInput: 0x03,
  WindowResize: 0x04,
  SessionControl: 0x05,
} as const;

export interface ParsedFrame {
  type: number;
  sessionCount: number;
  sessions: SessionBlock[];
}

export interface SessionBlock {
  sessionId: number;
  length: number;
  data: Uint8Array;
}

export function parseFrame(buffer: ArrayBuffer): ParsedFrame | null {
  if (buffer.byteLength < 2) return null;

  const view = new DataView(buffer);
  const type = view.getUint8(0);
  const sessionCount = view.getUint8(1);
  let offset = 2;

  const sessions: SessionBlock[] = [];

  for (let i = 0; i < sessionCount; i++) {
    if (offset + 6 > buffer.byteLength) return null;

    const sessionId = view.getUint32(offset, false); // BigEndian
    offset += 4;
    const length = view.getUint16(offset, false);
    offset += 2;

    if (offset + length > buffer.byteLength) return null;

    const data = new Uint8Array(buffer, offset, length);
    sessions.push({ sessionId, length, data });
    offset += length;
  }

  return { type, sessionCount, sessions };
}