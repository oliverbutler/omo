(function () {
  let socket;
  let reconnectTimer;
  const reconnectInterval = 1000; // 1 second
  let currentVersion = localStorage.getItem("appVersion") || "";

  function connect() {
    socket = new WebSocket("ws://" + window.location.host + "/ws");

    socket.onopen = function () {
      console.log("WebSocket connected for dev reload");
      clearTimeout(reconnectTimer);
    };

    socket.onclose = function () {
      console.log("WebSocket closed. Reconnecting...");
      reconnectTimer = setTimeout(connect, reconnectInterval);
    };

    socket.onmessage = function (event) {
      console.log("Received message from server:", event.data);
      if (event.data !== currentVersion) {
        currentVersion = event.data;
        localStorage.setItem("appVersion", currentVersion);
        window.location.reload();
      }
    };

    socket.onerror = function (error) {
      console.error("WebSocket error:", error);
    };
  }

  connect();
})();
