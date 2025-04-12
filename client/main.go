// client.go
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
)

type ProtocolMessage struct {
	Protocol   string
	Hostname   string
	AuthToken  string
	RemoteAddr string
}

func sendMessage(conn net.Conn, message ProtocolMessage) error {
	messageStr := fmt.Sprintf(
		"PROTOCOL: %s\nHOSTNAME: %s\nREMOTEADDR: %s\nAUTHTOKEN: %s\n",
		message.Protocol, message.Hostname, message.RemoteAddr, message.AuthToken,
	)
	payload := []byte(messageStr)
	length := uint32(len(payload))

	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}

	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}
	return nil
}

func main() {
	conn, err := net.Dial("tcp", "localhost:9000")
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	log.Println("Connected to server")

	message := ProtocolMessage{
		Protocol:   "http",
		Hostname:   "localhost:3000",
		AuthToken:  "XXXXXXXXXXXX",
		RemoteAddr: "127.0.0.1:8080",
	}
	if err := sendMessage(conn, message); err != nil {
		log.Fatal("Failed to send protocol message:", err)
	}

	reader := bufio.NewReader(conn)

	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			log.Println("Error reading request:", err)
			break
		}

		req.RequestURI = ""
		req.URL.Scheme = "http"
		req.URL.Host = "localhost:3000"
		fmt.Printf("Forwarding request: %s %s\n", req.Method, req.URL.String())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println("Failed to forward request:", err)
			break
		}

		if err := resp.Write(conn); err != nil {
			log.Println("Failed to write response:", err)
			break
		}
	}
}
