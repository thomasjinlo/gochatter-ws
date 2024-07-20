package gochatterclient

import (
	"context"
	"errors"
	"gochatter-ws/internal/connection"
	"log"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Manager struct {
	cm     *connection.Manager
	rc     *redis.Client
	domain string
}

func NewManager(cm *connection.Manager, rc *redis.Client, domain string) *Manager {
	return &Manager{
		cm:     cm,
		rc:     rc,
		domain: domain,
	}
}

func (m *Manager) SetupConnection(conn *websocket.Conn, clientId string) error {
	ctx := context.Background()
	if !m.cm.HasConnections(clientId) {
		err := m.rc.SAdd(ctx, clientId, m.domain).Err()
		if err != nil {
			return err
		}
	}
	go func() {
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				log.Printf("error from client connection: %v", err)
				break
			}
		}
		m.cm.RemoveConnection(clientId, conn)
		if !m.cm.HasConnections(clientId) {
			if err := m.rc.SRem(ctx, clientId, m.domain).Err(); err != nil {
				log.Printf("error while removing account/hostip mapping: %v", err)
			}
		}
	}()
	m.cm.AddConnection(clientId, conn)
	return nil
}

type directMessageRequest struct {
	Author  string
	Content string
}

func (m *Manager) SendDirectMessage(sourceClientId, targetClientId, msg string) error {
	var errs error
	conns := m.cm.GetConnections(targetClientId)
	for _, conn := range conns {
		err := conn.WriteJSON(directMessageRequest{
			Author:  sourceClientId,
			Content: msg,
		})
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}
