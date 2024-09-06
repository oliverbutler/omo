package main

import (
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	versionString string
	versionMutex  sync.RWMutex
)

func generateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, n)
	for i := range result {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}

func versionWsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade WebSocket", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	versionMutex.RLock()
	currentVersion := versionString
	versionMutex.RUnlock()

	err = conn.WriteMessage(websocket.TextMessage, []byte(currentVersion))
	if err != nil {
		// Handle error (e.g., log it)
		return
	}

	// Keep the connection open
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func InitDevReloadWebsocket(r *chi.Mux) {
	versionString = generateRandomString(10)

	r.Get("/ws", versionWsHandler)

	// Start a goroutine to update the version string periodically
	go func() {
		for {
			time.Sleep(5 * time.Second) // Wait for 5 seconds
			versionMutex.Lock()
			versionString = generateRandomString(10)
			versionMutex.Unlock()
		}
	}()
}
