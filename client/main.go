// client/main.go
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/xtaci/smux"
)

func main() {
	hostname := flag.String("hostname", "AUTO", "Subdomain to register or 'AUTO' to let server generate one")
	local := flag.String("local", "localhost:3000", "Local service to forward to")
	server := flag.String("server", "172.207.27.146:9000", "Tunnel server address")
	reconnectDelay := flag.Duration("reconnect-delay", 5*time.Second, "Delay between reconnection attempts")
	preserveClientIP := flag.Bool("preserve-ip", true, "Preserve original client IP in X-Forwarded-For header")
	flag.Parse()

	// Setup graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})

	go func() {
		<-signals
		log.Println("Shutting down client…")
		close(stop)
		time.Sleep(250 * time.Millisecond)
		os.Exit(0)
	}()

	// Connection loop with reconnect logic
	for {
		select {
		case <-stop:
			return
		default:
			assignedHostname, err := connectAndServe(*hostname, *local, *server, *preserveClientIP)
			if err != nil {
				log.Printf("Connection error: %v. Reconnecting in %v…", err, *reconnectDelay)
				select {
				case <-stop:
					return
				case <-time.After(*reconnectDelay):
				}
			} else {
				log.Printf("Server closed connection for hostname '%s'. Reconnecting...", assignedHostname)
				select {
				case <-stop:
					return
				case <-time.After(*reconnectDelay):
				}
			}
		}
	}
}

func connectAndServe(hostname, local, server string, preserveClientIP bool) (string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	log.Printf("Connected to server at %s. Registering with hostname request: '%s'", server, hostname)
	conn.SetDeadline(time.Time{}) // clear any deadlines

	// Send hostname request followed by newline
	if _, err := fmt.Fprintf(conn, "%s\n", hostname); err != nil {
		return "", fmt.Errorf("failed to send hostname request: %w", err)
	}

	assignedHostname := hostname
	if hostname == "AUTO" {
		reader := bufio.NewReader(conn)
		assignedHostname, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read assigned hostname: %w", err)
		}
		assignedHostname = strings.TrimSpace(assignedHostname)
		log.Printf("✨ Server assigned hostname: %s", assignedHostname)
		log.Printf("🌐 Your service is now available at: http://%s", assignedHostname)
	}

	// Create a smux client session
	session, err := smux.Client(conn, nil)
	if err != nil {
		return assignedHostname, fmt.Errorf("failed to create smux session: %w", err)
	}
	defer session.Close()

	// Accept and handle new streams concurrently.
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return assignedHostname, fmt.Errorf("failed to accept stream: %w", err)
		}
		go handleStream(stream, local, preserveClientIP)
	}
}

func handleStream(stream net.Conn, local string, preserveClientIP bool) {
	defer stream.Close()

	// Read the 4-byte length header for the HTTP request
	header := make([]byte, 4)
	if _, err := io.ReadFull(stream, header); err != nil {
		log.Println("Error reading stream header:", err)
		return
	}
	reqLen := binary.BigEndian.Uint32(header)
	reqBytes := make([]byte, reqLen)
	if _, err := io.ReadFull(stream, reqBytes); err != nil {
		log.Println("Error reading stream request:", err)
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(reqBytes)))
	if err != nil {
		log.Println("Error parsing HTTP request:", err)
		return
	}

	// Preserve client IP if available
	clientIP := req.Header.Get("X-Forwarded-For")
	remoteAddr := req.RemoteAddr
	req.RequestURI = ""
	req.URL.Scheme = "http"
	req.URL.Host = local

	if preserveClientIP && clientIP != "" {
		log.Printf("Preserving client IP: %s", clientIP)
	}

	log.Printf("Forwarding: %s %s (from %s)", req.Method, req.URL.String(), func() string {
		if clientIP != "" {
			return clientIP
		}
		return remoteAddr
	}())

	// Forward the request to the local Next.js server
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Local forward failed:", err)
		resp = &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("Failed to forward to local service")),
			Header:     make(http.Header),
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
	}

	// Write the response back to the stream with a 4-byte length header.
	var buf bytes.Buffer
	if err := resp.Write(&buf); err != nil {
		log.Println("Error encoding response:", err)
		return
	}
	respBytes := buf.Bytes()
	respLen := uint32(len(respBytes))
	lengthHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthHeader, respLen)

	// Set a write deadline for safety.
	stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := stream.Write(append(lengthHeader, respBytes...)); err != nil {
		log.Println("Error sending response on stream:", err)
		return
	}
	stream.SetWriteDeadline(time.Time{})
}
