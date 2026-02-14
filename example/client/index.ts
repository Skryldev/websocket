// client With Bun Js
const ws = new WebSocket("ws://localhost:8080/ws")

ws.onopen = () => {
  console.log("Connected to server")

  // join room
  ws.send(JSON.stringify({
    topic: "chat:join",
    event: "join",
    data: {
      room: "room1",
      user: "Ali"
    }
  }))

  // send a chat message
  ws.send(JSON.stringify({
    topic: "chat:room1",
    event: "message",
    data: {
      room: "room1",
      user: "Ali",
      text: "Hello World 🚀"
    }
  }))
}

ws.onmessage = (e) => {
  console.log("Received from server:", e.data)
}

ws.onclose = () => {
  console.log("Disconnected from server")
}

ws.onerror = (err) => {
  console.error("WebSocket error:", err)
}