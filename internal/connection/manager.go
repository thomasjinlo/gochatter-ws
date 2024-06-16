package connection

import "github.com/gorilla/websocket"

type Manager struct {
	connections map[string]map[string]*websocket.Conn
}

func NewManager() *Manager {
	return &Manager{
		connections: make(map[string]map[string]*websocket.Conn),
	}
}

func (m *Manager) GetConnections(accountId string) []*websocket.Conn {
	var conns []*websocket.Conn
	if clientConns, ok := m.connections[accountId]; ok {
		for _, conn := range clientConns {
			conns = append(conns, conn)
		}
	}
	return conns
}

func (m *Manager) HasConnections(accountId string) bool {
	if conns, present := m.connections[accountId]; !present {
		return false
	} else {
		return len(conns) > 0
	}
}

func (m *Manager) AddConnection(accountId string, conn *websocket.Conn) {
	if !m.HasConnections(accountId) {
		m.connections[accountId] = make(map[string]*websocket.Conn)
	}
	conns, _ := m.connections[accountId]
	conns[conn.RemoteAddr().String()] = conn

}

func (m *Manager) RemoveConnection(accountId string, conn *websocket.Conn) {
	var conns map[string]*websocket.Conn
	if m.HasConnections(accountId) {
		conns, _ = m.connections[accountId]
		delete(conns, conn.RemoteAddr().String())
	}
}
