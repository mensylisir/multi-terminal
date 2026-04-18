import { parseFrame } from './parser';

interface WorkerMessage {
  type: 'send' | 'connect' | 'disconnect';
  payload?: any;
}

let ws: WebSocket | null = null;

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

    case 'disconnect':
      if (ws) {
        ws.close();
        ws = null;
      }
      break;
  }
};