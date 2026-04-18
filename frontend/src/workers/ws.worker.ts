import { parseFrame, FrameType } from './parser';

interface WorkerMessage {
  type: 'send' | 'connect' | 'disconnect' | 'resize';
  payload?: any;
}

let ws: WebSocket | null = null;

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

self.onmessage = (e: MessageEvent<WorkerMessage>) => {
  const { type, payload } = e.data;

  switch (type) {
    case 'connect':
      ws = new WebSocket(payload.url);
      ws.binaryType = 'arraybuffer';
      ws.onmessage = (event) => {
        const frame = parseFrame(event.data);
        if (frame) {
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