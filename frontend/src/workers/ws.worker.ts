import { parseFrame, FrameType } from './parser';

// Confirm frame type for risk confirmation requests
const CONFIRM_FRAME_TYPE = 0x08;

interface WorkerMessage {
  type: 'send' | 'connect' | 'disconnect' | 'resize' | 'input';
  payload?: any;
}

let ws: WebSocket | null = null;

// EchoLock management
const echoLockSessions = new Set<number>();
const pendingEchoFrames: Map<number, Uint8Array[]> = new Map();

function createResizeFrame(sessionId: number, cols: number, rows: number): ArrayBuffer {
  const headerSize = 1 + 1 + 4 + 2 + 2; // FrameType + SessionCount + SessionId + Cols + Rows
  const buf = new ArrayBuffer(headerSize);
  const view = new DataView(buf);
  view.setUint8(0, FrameType.WindowResize); // 0x04
  view.setUint8(1, 1); // SessionCount
  view.setUint32(2, sessionId, false); // BigEndian
  view.setUint16(6, cols, false); // BigEndian
  view.setUint16(8, rows, false); // BigEndian
  return buf;
}

function handleInput(frame: { sessions: { sessionId: number; data: Uint8Array }[] }) {
  // Set EchoLock for all sessions in the input
  for (const block of frame.sessions) {
    echoLockSessions.add(block.sessionId);
    pendingEchoFrames.set(block.sessionId, []);
  }

  // 50ms 后释放锁
  setTimeout(() => {
    for (const block of frame.sessions) {
      echoLockSessions.delete(block.sessionId);
      // 处理积压的帧
      const pending = pendingEchoFrames.get(block.sessionId) || [];
      for (const data of pending) {
        self.postMessage({ type: 'output', sessionId: block.sessionId, data });
      }
      pendingEchoFrames.delete(block.sessionId);
    }
  }, 50);
}

self.onmessage = (e: MessageEvent<WorkerMessage>) => {
  const { type, payload } = e.data;

  switch (type) {
    case 'connect':
      ws = new WebSocket(payload.url);
      ws.binaryType = 'arraybuffer';
      ws.onmessage = (event) => {
        const frame = parseFrame(event.data);
        if (frame) {
          // Check if this is a confirm frame (risk confirmation request)
          if (frame.type === CONFIRM_FRAME_TYPE) {
            // Parse confirmation request and forward to main thread
            const confirmData = JSON.parse(new TextDecoder().decode(frame.sessions[0]?.data));
            self.postMessage({ type: 'confirm', payload: confirmData });
            return;
          }
          // Check if this is an input echo (should be locked)
          if (frame.type === FrameType.TerminalOutput) {
            for (const block of frame.sessions) {
              if (echoLockSessions.has(block.sessionId)) {
                // Queue the frame instead of delivering
                const pending = pendingEchoFrames.get(block.sessionId) || [];
                pending.push(block.data);
                pendingEchoFrames.set(block.sessionId, pending);
                continue;
              }
            }
          }
          self.postMessage({ type: 'frame', payload: frame });
        }
      };
      ws.onclose = () => {
        self.postMessage({ type: 'closed' });
      };
      break;

    case 'send':
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(payload);
      }
      break;

    case 'input':
      handleInput(payload);
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(payload);
      }
      break;

    case 'resize':
      if (ws && ws.readyState === WebSocket.OPEN) {
        const frame = createResizeFrame(payload.sessionId, payload.cols, payload.rows);
        ws.send(frame);
      }
      break;

    case 'disconnect':
      if (ws) {
        ws.close();
        ws = null;
      }
      break;
  }
};