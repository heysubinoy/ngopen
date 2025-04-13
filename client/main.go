package main

import (
	"bufio"
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
)

func main() {
	hostname := flag.String("hostname", "AUTO", "Subdomain to register or 'AUTO' to let server generate one")
	local := flag.String("local", "localhost:3000", "Local service to forward to")
	server := flag.String("server", "localhost:9000", "Tunnel server address")
	reconnectDelay := flag.Duration("reconnect-delay", 5*time.Second, "Delay between reconnection attempts")
	flag.Parse()

	// Setup signal handling for graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Create a stop channel to terminate reconnection loop
	stop := make(chan struct{})

	go func() {
		<-signals
		log.Println("Shutting down client...")
		close(stop)
		// Give some time for clean disconnection before exiting
		time.Sleep(250 * time.Millisecond)
		os.Exit(0)
	}()

	// Connection loop with reconnect logic
	for {
		select {
		case <-stop:
			return
		default:
			if assignedHostname, err := connectAndServe(*hostname, *local, *server); err != nil {
				log.Printf("Connection error: %v. Reconnecting in %v...", err, *reconnectDelay)
				select {
				case <-stop:
					return
				case <-time.After(*reconnectDelay):
					// Continue the loop to reconnect
				}
			} else {
				// If we get here, connection closed without error (server closed it)
				log.Printf("Server closed connection for hostname '%s'. Reconnecting...", assignedHostname)
				select {
				case <-stop:
					return
				case <-time.After(*reconnectDelay):
					// Continue the loop to reconnect
				}
			}
		}
	}
}

func connectAndServe(hostname, local, server string) (string, error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return "", fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	log.Printf("Connected to server at %s. Registering with hostname request: '%s'\n", server, hostname)

	// Set connection deadlines to detect network issues more quickly
	conn.SetDeadline(time.Time{}) // Clear any deadline

	// Send hostname request (or AUTO) followed by newline
	if _, err := fmt.Fprintf(conn, "%s\n", hostname); err != nil {
		return "", fmt.Errorf("failed to send hostname request: %w", err)
	}

	// If hostname was AUTO, read the assigned hostname from server
	assignedHostname := hostname
	if hostname == "AUTO" {
		reader := bufio.NewReader(conn)
		assignedHostname, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read assigned hostname: %w", err)
		}
		assignedHostname = strings.TrimSpace(assignedHostname)
		log.Printf("✨ Server assigned hostname: %s\n", assignedHostname)
		log.Printf("🌐 Your service is now available at: http://%s\n", assignedHostname)
	}

	// Send periodic heartbeats to detect disconnection
	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)

	// Main request handling loop
	for {
		// Set read deadline to detect broken connections
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			return assignedHostname, fmt.Errorf("failed to set read deadline: %w", err)
		}

		// Read 4-byte length prefix
		header := make([]byte, 4)
		_, err := io.ReadFull(conn, header)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// This was just a timeout, continue the loop
				continue
			}
			return assignedHostname, fmt.Errorf("error reading length: %w", err)
		}

		// Clear deadline after successful read
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return assignedHostname, fmt.Errorf("failed to clear read deadline: %w", err)
		}

		reqLen := binary.BigEndian.Uint32(header)
		reqBytes := make([]byte, reqLen)
		_, err = io.ReadFull(conn, reqBytes)
		if err != nil {
			return assignedHostname, fmt.Errorf("error reading request: %w", err)
		}

		req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(string(reqBytes))))
		if err != nil {
			log.Println("Error parsing request:", err)
			continue
		}

		req.RequestURI = ""
		req.URL.Scheme = "http"
		req.URL.Host = local

		log.Printf("Forwarding: %s %s\n", req.Method, req.URL.String())

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

		var b strings.Builder
		err = resp.Write(&b)
		if err != nil {
			log.Println("Error encoding response:", err)
			continue
		}
		respBytes := []byte(b.String())

		respLen := uint32(len(respBytes))
		lengthHeader := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthHeader, respLen)

		// Set write deadline for sending the response
		if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return assignedHostname, fmt.Errorf("failed to set write deadline: %w", err)
		}

		_, err = conn.Write(append(lengthHeader, respBytes...))

		// Clear deadline after write
		if err := conn.SetWriteDeadline(time.Time{}); err != nil {
			return assignedHostname, fmt.Errorf("failed to clear write deadline: %w", err)
		}

		if err != nil {
			return assignedHostname, fmt.Errorf("error sending response: %w", err)
		}
	}
}
