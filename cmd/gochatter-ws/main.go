package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

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
	r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[gochatter-ws] received hello world request")
		w.Write([]byte("Hello, World!"))
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/connect", setupConnection(upgrader, connections))
	r.Post("/send_message", broadcast(connections))

	return r
}

func setupConnection(u websocket.Upgrader, conns map[string]Connection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[gochatter-ws] setting up connection for client %v", r.RemoteAddr)
		c, err := u.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}
		conn := Connection{
			clientId: c.RemoteAddr().String(),
			conn:     c,
		}
		conns[conn.clientId] = conn
	}
}

type SendMessageBody struct {
	Author  string
	Content string
}

func broadcast(conns map[string]Connection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("[gochatter-ws] broadcasting message")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var msg SendMessageBody
		if err := json.Unmarshal(b, &msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var pushErrs error
		for _, c := range conns {
			if c.clientId == msg.Author {
				continue
			}
			err := c.conn.WriteJSON(msg)
			if err != nil {
				errors.Join(pushErrs, err)
			}
		}

		if pushErrs != nil {
			http.Error(w, pushErrs.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	log.Print("[gochatter-ws] starting up GoChatter Websocket Server on port 8444")
	mux := setupRoutes()
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServeTLS(
		":8444",
		filepath.Join(root, ".credentials", "cert.pem"),
		filepath.Join(root, ".credentials", "key.pem"),
	mux))
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
