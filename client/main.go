// client.go
package main

import (
	"bufio"
	"encoding/binary"
	"flag"
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
	protocol := flag.String("protocol", "http", "Protocol to use (e.g., http)")
	hostname := flag.String("hostname", "", "Local hostname to expose")
	authToken := flag.String("auth", "", "Authentication token")
	// serverAddr := flag.String("server", "", "NGOpen server address")
	// remoteAddr := flag.String("remote", "localhost:8080", "Remote address to forward to")
	
	remoteAddr := "localhost:8080" // Default remote address
	serverAddr := "localhost:9000" // Default server address


	flag.Parse()
	// var serverAddr="localhost:9000"
	// if *serverAddr == "" {
	// 	log.Fatal("Server address is required. Use -server flag")
	// }
	if *hostname == "" {
		log.Fatal("Hostname is required. Use -hostname flag")
	}
	if *protocol == "" {
		log.Fatal("Protocol is required. Use -protocol flag")
	}
	if *authToken == "" {
		log.Fatal("Authentication token is required. Use -auth flag")
	}

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	log.Println("Connected to server")

	message := ProtocolMessage{
		Protocol:   *protocol,
		Hostname:   *hostname,
		AuthToken:  *authToken,
		RemoteAddr: remoteAddr,
	}
	fmt.Printf("Sending protocol message: %s\n", message)
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
