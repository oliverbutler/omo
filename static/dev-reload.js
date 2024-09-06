const WS_URL = 'ws://localhost:6900/ws';
let socket;
let currentVersion = null;

function connectWebSocket() {
  socket = new WebSocket(WS_URL);

  socket.onopen = () => {
    console.debug('WebSocket connection opened:');
  };

  socket.onmessage = (event) => {
    const newVersion = event.data;

    if (!currentVersion) {
      currentVersion = newVersion;
    } else if (currentVersion !== newVersion) {
      currentVersion = newVersion;
      window.location.reload();
    }
  };

  socket.onclose = () => {
    console.debug('WebSocket closed. Attempting to reconnect...');
    setTimeout(connectWebSocket, 250);
  };

  socket.onerror = (event) => {
    console.debug('WebSocket error:', event);
  };
}

// Initially establish the WebSocket connection
connectWebSocket();
