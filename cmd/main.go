package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type Connection struct {
	clientId string
	conn     *websocket.Conn
}

func setupRoutes() *chi.Mux {
	connections := make(map[string]Connection)
	upgrader := websocket.Upgrader{}

	r := chi.NewRouter()
	r.Get("/connect", setupConnection(upgrader, connections))

	return r
}

func setupConnection(u websocket.Upgrader, conns map[string]Connection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[gochatter ws] Setting up connection for client %v", r.RemoteAddr)
		c, err := u.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer c.Close()
		conn := Connection{
			clientId: c.RemoteAddr().String(),
			conn:     c,
		}
		conns[conn.clientId] = conn
	}
}

func main() {
	log.Print("[gochatter ws] Starting up GoChatter Websocket Server on port 8443")
	mux := setupRoutes()
	switch os.Args[1] {
	case "client":
		createClient()
	default:
		log.Fatal(http.ListenAndServe(":8443", mux))
	}
}

func createClient() {
	c, _, err := websocket.DefaultDialer.Dial("ws://localhost:8443/connect", nil)
	if err != nil {
		log.Fatal(err)
	}

	defer c.Close()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println("Enter message to WS: ")
		scanner.Scan()
		input := scanner.Text()
		c.WriteMessage(websocket.TextMessage, []byte(input))
	}
}
