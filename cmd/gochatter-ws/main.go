package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Connection struct {
	clientId string
	conn     *websocket.Conn
}

func setupRoutes() *chi.Mux {
	connections := make(map[string]*Connection)
	upgrader := websocket.Upgrader{}

	r := chi.NewRouter()
	r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		log.Print("[gochatter-ws] received hello world request")
		w.Write([]byte("Hello, World!"))
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/connect", setupConnection(upgrader, connections))
	r.Post("/broadcast", broadcast(connections))
	r.Post("/direct_message", sendDirectMessage(connections))

	return r
}

type DirectMessageBody struct {
	AccountId string
	Content   string
}

func sendDirectMessage(conns map[string]*Connection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Print("[gochatter-ws] sending direct message")
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			msg := fmt.Sprintf("invalid content type %s, expected \"application/json\"", ct)
			http.Error(w, msg, http.StatusUnsupportedMediaType)
			return
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var dm DirectMessageBody
		if err := json.Unmarshal(b, &dm); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if c, ok := conns[dm.AccountId]; ok {
			c.conn.WriteJSON([]byte(dm.Content))
		}

		w.WriteHeader(http.StatusOK)
	}
}

func setupConnection(u websocket.Upgrader, conns map[string]*Connection) http.HandlerFunc {
	ctx := context.Background()
	rc := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("[gochatter-ws] failed to retrieve hostname: %v", err)
	}
	ipAddr, err := net.ResolveIPAddr("ip", hostname)
	if err != nil {
		log.Fatalf("[gochatter-ws] failed to retrieve host ip: %v", err)
	}
	hostIp := ipAddr.IP.String()
	log.Printf("[gochatter-ws] serving on host ip: %v", hostIp)

	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[gochatter-ws] setting up connection for client %v", r.RemoteAddr)
		c, err := u.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}
		conn := &Connection{
			clientId: c.RemoteAddr().String(),
			conn:     c,
		}
		conns[conn.clientId] = conn

		log.Printf("[gochatter-ws] setting up connection for client %v to host %v", conn.clientId, hostIp)
		if err = rc.Set(ctx, conn.clientId, hostIp, 100*time.Millisecond).Err(); err != nil {
			log.Printf("[gochatter-ws] error setting redis %v", err)
		}
	}
}

type SendMessageBody struct {
	Author  string
	Content string
}

func broadcast(conns map[string]*Connection) http.HandlerFunc {
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
	log.Printf("[gochatter-ws] root path %s", root)

	log.Fatal(http.ListenAndServeTLS(
		":8444",
		filepath.Join(root, ".credentials", "cert.pem"),
		filepath.Join(root, ".credentials", "key.pem"),
		mux))
}
