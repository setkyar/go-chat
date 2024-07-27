package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	conn *websocket.Conn
}

var clients = make(map[*Client]bool)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var redisClient = redis.NewClient(&redis.Options{
	Addr: "localhost:6379", // Update with your Redis instance details
})

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ws.Close()

	// Add client to the map
	client := &Client{conn: ws}
	clients[client] = true

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			// Remove client from the map when they disconnect
			delete(clients, client)
			fmt.Println(err)
			break
		}

		// Publish message to Redis
		err = redisClient.Publish(context.Background(), "chatroom", string(msg)).Err()
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}

func listenToRedisChannel() {
	pubsub := redisClient.Subscribe(context.Background(), "chatroom")
	ch := pubsub.Channel()

	for msg := range ch {
		broadcastToClients(msg.Payload)
	}
}

func broadcastToClients(message string) {
	for client := range clients {
		err := client.conn.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			fmt.Println(err)
			client.conn.Close()
			delete(clients, client)
		}
	}
}

func main() {
	// Start listening to Redis channel in a new Goroutine
	go listenToRedisChannel()

	// WebSocket route
	http.HandleFunc("/ws", handleConnections)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "client/index.html")
	})

	// Serve static assets
	fs := http.FileServer(http.Dir("client"))
	http.Handle("/client/", http.StripPrefix("/client/", fs))

	// Start the server
	fmt.Println("Server started on :8000")
	err := http.ListenAndServe("0.0.0.0:8000", nil)
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
	}
}
