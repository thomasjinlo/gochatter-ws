package connection

import (
	"context"
	"errors"
	"log"

	"github.com/gorilla/websocket"
)

type Registry interface {
	Add(ctx context.Context, clientId, endpoint string) error
	Remove(ctx context.Context, clientId, endpoint string) error
}

type Manager struct {
	connections map[string]map[string]*websocket.Conn
	registry Registry
	endpoint string
}

func NewManager(endpoint string, registry Registry) *Manager {
	return &Manager{
		endpoint: endpoint,
		registry: registry,
		connections: make(map[string]map[string]*websocket.Conn),
	}
}

func (m *Manager) SetupConnection(conn *websocket.Conn, clientId string) error {
	ctx := context.Background()
	if !m.hasConnections(clientId) {
		if err := m.registry.Add(ctx, clientId, m.endpoint); err != nil {
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
		m.removeConnection(clientId, conn)
		if !m.hasConnections(clientId) {
			if err := m.registry.Remove(ctx, clientId, m.endpoint); err != nil {
				log.Printf("error while removing account/hostip mapping: %v", err)
			}
		}
	}()
	m.addConnection(clientId, conn)
	return nil
}

type directMessageRequest struct {
	Author  string
	Content string
}

func (m *Manager) SendDirectMessage(sourceClientId, targetClientId, msg string) error {
	var errs error
	conns := m.getConnections(targetClientId)
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

func (m *Manager) getConnections(accountId string) []*websocket.Conn {
	var conns []*websocket.Conn
	if clientConns, ok := m.connections[accountId]; ok {
		for _, conn := range clientConns {
			conns = append(conns, conn)
		}
	}
	return conns
}

func (m *Manager) hasConnections(accountId string) bool {
	if conns, present := m.connections[accountId]; !present {
		return false
	} else {
		return len(conns) > 0
	}
}

func (m *Manager) addConnection(accountId string, conn *websocket.Conn) {
	if !m.hasConnections(accountId) {
		m.connections[accountId] = make(map[string]*websocket.Conn)
	}
	conns, _ := m.connections[accountId]
	conns[conn.RemoteAddr().String()] = conn

}

func (m *Manager) removeConnection(accountId string, conn *websocket.Conn) {
	var conns map[string]*websocket.Conn
	if m.hasConnections(accountId) {
		conns, _ = m.connections[accountId]
		delete(conns, conn.RemoteAddr().String())
	}
}
