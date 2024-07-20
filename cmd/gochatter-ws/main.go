package main

import (
	"crypto/tls"
	"gochatter-ws/internal/connection"
	"gochatter-ws/internal/gochatterclient"
	"gochatter-ws/internal/handlers"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Connection struct {
	accountId string
	clientId  string
	conn      *websocket.Conn
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
	hostip := ipAddr.IP.String()
	log.Printf("[gochatter-ws] serving on host ip: %v", hostip)
	names, err := net.LookupAddr(ipAddr.String())
	if err != nil {
		log.Fatalf("[gochatter-ws] failed to retrieve host FQDN: %v", err)
	}
	fqdn := names[0]
	log.Printf("[gochatter-ws] host FQDN: %v", fqdn)
	rc := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})
	connManager := connection.NewManager()
	cm := gochatterclient.NewManager(connManager, rc, fqdn)
	mux := handlers.SetupRoutes(cm)
	publicCert, err := tls.LoadX509KeyPair(
		os.Getenv("PUBLIC_CERT_PATH"),
		os.Getenv("PUBLIC_KEY_PATH"),
	)
	if err != nil {
		log.Fatal(err)
	}
	privateCert, err := tls.LoadX509KeyPair(
		os.Getenv("PRIVATE_CERT_PATH"),
		os.Getenv("PRIVATE_KEY_PATH"),
	)
	getCertificate := func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		switch info.ServerName {
		case "websockets.gochatter.app":
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
