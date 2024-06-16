package message

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

type Handle struct {
	service  *Service
	upgrader *websocket.Upgrader
}

func NewHandle(service *Service, upgrader *websocket.Upgrader) *Handle {
	return &Handle{
		service:  service,
		upgrader: upgrader,
	}
}

type DirectMessageRequest struct {
	SourceAccountId string
	TargetAccountId string
	Content         string
}

func (h *Handle) DirectMessage(w http.ResponseWriter, r *http.Request) {
	slog.Info("handling direct message")
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
	h.service.DirectMessage(dm)
	w.WriteHeader(http.StatusOK)
}

func (h *Handle) Connect(w http.ResponseWriter, r *http.Request) {
	slog.Info(fmt.Sprintf("[gochatter-ws] setting up connection for client %v", r.RemoteAddr))
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	accountId := r.Header.Get("AccountId")
	if accountId == "" {
		http.Error(w, "hello", http.StatusBadRequest)
		return
	}
	if err := h.service.SetupConnection(accountId, conn); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
