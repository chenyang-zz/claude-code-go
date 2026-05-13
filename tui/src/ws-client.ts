export interface WSMessage {
  type: string;
  payload?: any;
}

export type EventHandler = (msg: WSMessage) => void;

export function connectWS(port: number, onEvent: EventHandler): WebSocket {
  const url = `ws://127.0.0.1:${port}/ws`;
  const ws = new WebSocket(url);

  ws.onopen = () => {
    // Connected
  };

  ws.onmessage = (event: MessageEvent) => {
    try {
      const msg: WSMessage = JSON.parse(event.data as string);
      onEvent(msg);
    } catch {
      // Ignore parse errors
    }
  };

  ws.onclose = () => {
    // Connection closed
  };

  ws.onerror = () => {
    // Connection error
  };

  return ws;
}

export function sendInput(ws: WebSocket, text: string): void {
  if (ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({
    type: "input",
    payload: { text },
  }));
}

export function sendApproval(ws: WebSocket, approved: boolean): void {
  if (ws.readyState !== WebSocket.OPEN) return;
  ws.send(JSON.stringify({
    type: "approval",
    payload: { approved },
  }));
}
