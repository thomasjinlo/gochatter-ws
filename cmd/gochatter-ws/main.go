package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"gochatter-ws/internal/connection"
	"gochatter-ws/internal/message"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Connection struct {
	accountId string
	clientId  string
	conn      *websocket.Conn
}

func setupRoutes(handle *message.Handle) *chi.Mux {
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
	// r.Post("/direct_message", sendDirectMessage(connections))

	r.Get("/connect", handle.Connect)
	r.Post("/direct_message", handle.DirectMessage)

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

type ConnectBody struct {
	AccountId string
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
		defer c.Close()
		var cb ConnectBody
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "error reading body", http.StatusInternalServerError)
			return
		}
		if err = json.Unmarshal(b, &cb); err != nil {
			http.Error(w, "error parsing body", http.StatusBadRequest)
			return
		}
		conn := &Connection{
			accountId: cb.AccountId,
			clientId:  c.RemoteAddr().String(),
			conn:      c,
		}
		conns[cb.AccountId] = conn

		log.Printf("[gochatter-ws] setting up connection for client %v to host %v", conn.clientId, hostIp)
		if err = rc.Set(ctx, conn.clientId, hostIp, 0).Err(); err != nil {
			log.Printf("[gochatter-ws] error setting redis %v", err)
		}

		for {
			_, _, err := conn.conn.ReadMessage()
			if err != nil {
				log.Printf("[gochatter-ws] client connection closed for account %s and client ip %s", conn.accountId, conn.clientId)
				rc.Del(ctx, conn.clientId)
				delete(conns, conn.accountId)
				break
			}
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
	rc := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})
	cm := connection.NewManager()
	service := message.NewService(rc, cm, hostIp)
	upgrader := websocket.Upgrader{}
	handle := message.NewHandle(service, &upgrader)
	mux := setupRoutes(handle)
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	publicCert, err := tls.LoadX509KeyPair(
		filepath.Join(root, ".credentials", "cert.pem"),
		filepath.Join(root, ".credentials", "key.pem"),
	)
	if err != nil {
		log.Fatal(err)
	}
	privateCert, err := tls.LoadX509KeyPair(
		filepath.Join(root, ".credentials", "ws-cert.pem"),
		filepath.Join(root, ".credentials", "ws-cert.key"),
	)
	getCertificate := func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		switch info.ServerName {
		case "gochatter.app":
			return &publicCert, nil
		default:
			return &privateCert, nil
		}
	}
	config := &tls.Config{GetCertificate: getCertificate}
	server := &http.Server{
		Addr:      ":8444",
		TLSConfig: config,
		Handler:   mux,
	}
	log.Fatal(server.ListenAndServeTLS("", ""))
}
