package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

type User struct {
	displayName string
	conn *websocket.Conn
}

type ConnectBody struct {
	ChannelIds []string
	DisplayName string
}

type MessageBody struct {
	ChannelId string
	DisplayName string
	Message string
}

type PublishMessage struct {
	DisplayName string
	Message string
}

func SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	channelToUsers := make(map[string][]*User)
	upgrader := websocket.Upgrader{}
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}

	mux.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal(err)
		}

		displayName := r.Header.Get("DisplayName")
		channelIds := r.Header.Get("ChannelIds")

		user := &User{
			displayName: displayName,
			conn: conn,
		}

		for _, channelId := range strings.Split(channelIds, ",") {
			log.Println(fmt.Sprintf("[CONNECT]: Subscribing to channel %s", channelId))
			channelToUsers[channelId] = append(channelToUsers[channelId], user)
		}
	})

	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Fatal(err)
		}

		var messageBody MessageBody
		if err := json.Unmarshal(body, &messageBody); err != nil {
			log.Fatal(err)
		}

		pubMsg := PublishMessage{
			DisplayName: messageBody.DisplayName,
			Message: messageBody.Message,
		}
		data, err := json.Marshal(pubMsg)
		if err != nil {
			log.Fatal(err)
		}
		nc.Publish(fmt.Sprintf("channel.%s", messageBody.ChannelId), data)
	})

	go func() {
		nc.Subscribe("channel.*", func(m *nats.Msg) {
			subjects := strings.Split(m.Subject, ".")
			channelId := subjects[1]
			log.Println(fmt.Sprintf("[SUBSCRIBE]: received message from channel %s", channelId))
			log.Println(fmt.Sprintf("[SUBSCRIBE]: received data %v", string(m.Data)))
			for _, user := range channelToUsers[channelId] {
				var pubMsg PublishMessage
				if err := json.Unmarshal(m.Data, &pubMsg); err != nil {
					log.Fatal(err)
				}
				user.conn.WriteJSON(pubMsg)
			}
		})
	}()

	return mux
}

func main() {
	mux := SetupRoutes()
	err := http.ListenAndServe(":8443", mux)
	if err != nil {
		log.Fatal(err)
	}
}
