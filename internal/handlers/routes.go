package handlers

import (
	"encoding/json"
	"fmt"
	"gochatter-ws/internal/gochatterclient"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func SetupRoutes(cm *gochatterclient.Manager) *chi.Mux {
	u := &websocket.Upgrader{}
	r := chi.NewRouter()
	r.Use(loggingMiddleware)
	r.Get("/hello", handleHello())
	r.Get("/connect", handleConnect(cm, u))
	r.Post("/direct_message", handleDirectMessage(cm))
	return r
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[gochatter-ws] received HTTP method %s on path %s", r.Method, r.URL.String())
		next.ServeHTTP(w, r)
	})
}

func handleHello() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
		w.WriteHeader(http.StatusOK)
	}
}

func handleConnect(cm *gochatterclient.Manager, u *websocket.Upgrader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := u.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}
		accountId := r.Header.Get("AccountId")
		if accountId == "" {
			http.Error(w, "Missing AccountId", http.StatusBadRequest)
			return
		}
		if err := cm.SetupConnection(conn, accountId); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

type DirectMessageRequest struct {
	SourceAccountId string
	TargetAccountId string
	Content         string
}

func handleDirectMessage(cm *gochatterclient.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		var dm DirectMessageRequest
		if err := json.Unmarshal(b, &dm); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := cm.SendDirectMessage(dm.SourceAccountId, dm.TargetAccountId, dm.Content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
