package message

import (
	"context"
	"fmt"
	"gochatter-ws/internal/connection"
	"log/slog"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	fqdn     string
	hostname string
	hostip   string
	rc       *redis.Client
	cm       *connection.Manager
}

func NewService(rc *redis.Client, cm *connection.Manager, hostip, hostname, fqdn string) *Service {
	return &Service{
		rc:       rc,
		cm:       cm,
		hostip:   hostip,
		hostname: hostname,
		fqdn:     fqdn,
	}
}

type directMessage struct {
	Author  string
	Content string
}

func (s *Service) DirectMessage(dm DirectMessageRequest) {
	conns := s.cm.GetConnections(dm.TargetAccountId)
	for _, conn := range conns {
		err := conn.WriteJSON(directMessage{
			Author:  dm.SourceAccountId,
			Content: dm.Content,
		})
		if err != nil {
			slog.Info(fmt.Sprintf("error while sending dm: %v", err))
		}
	}
}

func (s *Service) SetupConnection(accountId string, conn *websocket.Conn) error {
	ctx := context.Background()
	if !s.cm.HasConnections(accountId) {
		err := s.rc.SAdd(ctx, accountId, s.fqdn).Err()
		if err != nil {
			return err
		}
	}
	go func() {
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				slog.Info(fmt.Sprintf("error from client connection: %v", err))
				break
			}
		}
		s.cm.RemoveConnection(accountId, conn)
		if !s.cm.HasConnections(accountId) {
			if err := s.rc.SRem(ctx, accountId, s.fqdn).Err(); err != nil {
				slog.Info(fmt.Sprintf("error while removing account/hostip mapping: %v", err))
			}
		}
	}()
	s.cm.AddConnection(accountId, conn)
	return nil
}
