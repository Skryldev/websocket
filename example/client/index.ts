// Run with: bun run client-bun.js

const ws = new WebSocket("ws://localhost:8080/ws");

ws.onopen = () => {
  console.log("✅ Connected to server");

  // 🌟 join room
  sendMessage("chat:join", "join", {
    room: "room1",
    user: "Ali"
  });

  // 🌟 send a chat message after 1 sec
  setTimeout(() => {
    sendMessage("chat:room1", "message", {
      room: "room1",
      user: "Ali",
      text: "Hello Bun.js 🚀"
    });
  }, 1000);

  // 🌟 leave room after 5 sec
  setTimeout(() => {
    sendMessage("chat:leave", "leave", {
      room: "room1",
      user: "Ali"
    });
  }, 5000);
};

// 📨 Receiving messages from server
ws.onmessage = (event) => {
  try {
    const msg = JSON.parse(event.data);
    console.log("📩 Received:", msg);
  } catch (err) {
    console.error("⚠️ Failed to parse message:", event.data);
  }
};

// 🔴 Connection closed
ws.onclose = () => {
  console.log("❌ Disconnected from server");
};

// ⚠️ Error handler
ws.onerror = (err) => {
  console.error("💥 WebSocket error:", err);
};


function sendMessage(topic: string, event: string, data: { room: string; user: string; text?: string; }) {
  const payload = {
    topic,
    event,
    data
  };
  ws.send(JSON.stringify(payload));
  console.log("📤 Sent:", payload);
}
