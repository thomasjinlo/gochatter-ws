package main

import (
	"crypto/tls"
	"gochatter-ws/internal/connection"
	"gochatter-ws/internal/handlers"
	"log"
	"net"
	"net/http"
	"os"
)

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


	var localhost bool
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-l", "--localhost":
			localhost = true
		}
	}

	registryEndpoint := "redis:6379" 
	serverEndpoint := fqdn
	if localhost {
		log.Print("[gochatter-ws] using localhost")
		registryEndpoint = "localhost:6379"
		serverEndpoint = "localhost"
	}

	pr := connection.NewRedisRegistry(registryEndpoint, "", 0)
	cm := connection.NewManager(serverEndpoint, pr)
	mux := handlers.SetupRoutes(cm)

	if localhost {
		log.Fatal(http.ListenAndServe(":8444", mux))
	} else {
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
}
