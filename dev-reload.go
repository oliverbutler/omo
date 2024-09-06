package main

import (
	"math/rand"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var versionString string

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

// versionWsHandler is a WebSocket handler that sends a random string to the client.
// This is used to force the client to reload when the server is restarted.
func versionWsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade WebSocket", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte(versionString))
	if err != nil {
		// Handle error (e.g., log it)
	}
}

func InitDevReloadWebsocket(r *chi.Mux) {
	versionString = generateRandomString(10)

	r.Get("/ws", versionWsHandler)
}
