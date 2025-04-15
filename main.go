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

// Custom logger with emoji prefixes
func logSuccess(format string, v ...interface{}) {
	log.Printf("✓ "+format, v...)
}

func logInfo(format string, v ...interface{}) {
	log.Printf("ℹ️ "+format, v...)
}

func logError(format string, v ...interface{}) {
	log.Printf("❌ "+format, v...)
}

func main() {
	hostname := flag.String("hostname", "AUTO", "Subdomain to register or 'AUTO' to let server generate one")
	local := flag.String("local", "", "Local service to forward to")
	server := flag.String("server", "172.207.27.146:9000", "Tunnel server address")
	reconnectDelay := flag.Duration("reconnect-delay", 5*time.Second, "Delay between reconnection attempts")
	preserveClientIP := flag.Bool("preserve-ip", true, "Preserve original client IP in X-Forwarded-For header")
	flag.Parse()
	if *hostname == "" || *local == "" {
		flag.Usage()
		os.Exit(1)
	}
	// Setup graceful shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})

	go func() {
		<-signals
		logInfo("Shutting down client...")
		close(stop)
		time.Sleep(250 * time.Millisecond)
		os.Exit(0)
	}()

	// Track the last assigned hostname to preserve it across reconnections
	lastAssignedHostname := *hostname

	// Connection loop with reconnect logic
	for {
		select {
		case <-stop:
			return
		default:
			assignedHostname, err := connectAndServe(lastAssignedHostname, *local, *server, *preserveClientIP)
			if err != nil {
				if assignedHostname != "" {
					// If we got a hostname before the error, preserve it
					lastAssignedHostname = assignedHostname
				}
				logError("Connection error: %v. Reconnecting to %s in %v...", 
					err, lastAssignedHostname, *reconnectDelay)
				select {
				case <-stop:
					return
				case <-time.After(*reconnectDelay):
				}
			} else if assignedHostname != "" {
				// If connection closed normally but we had a hostname, preserve it
				lastAssignedHostname = assignedHostname
				logInfo("Server closed connection for hostname '%s'. Reconnecting...", lastAssignedHostname)
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

	authToken := flag.String("auth-token", os.Getenv("NGOPEN_AUTH_TOKEN"), "Authentication token for server")
	flag.Parse()

	if *authToken == "" {
		return "", fmt.Errorf("authentication token is not set. Use --auth-token flag or set NGOPEN_AUTH_TOKEN environment variable")
	}

	logInfo("Connected to server at %s. Registering with hostname request: '%s'", server, hostname)
	conn.SetDeadline(time.Time{}) // clear any deadlines

	// Send auth token followed by hostname request
	if _, err := fmt.Fprintf(conn, "%s\n%s\n", *authToken, hostname); err != nil {
		return "", fmt.Errorf("failed to send auth token and hostname request: %w", err)
	}

	assignedHostname := hostname
	if hostname == "AUTO" {
		reader := bufio.NewReader(conn)
		assignedHostname, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read assigned hostname: %w", err)
		}
		assignedHostname = strings.TrimSpace(assignedHostname)
		
		// Cool format for successful connection
		fmt.Println("\n✓ Tunnel established")
		fmt.Printf("✓ Forwarding https://%s -> localhost:%s\n", assignedHostname, strings.TrimPrefix(local, "localhost:"))
		fmt.Println("✓ Ready for connections\n")
	} else {
		// Cool format for custom hostname
		fmt.Println("\n✓ Tunnel established")
		fmt.Printf("✓ Forwarding https://%s -> %s\n", hostname, local)
		fmt.Println("✓ Ready for connections\n")
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
		logError("Error reading stream header: %v", err)
		return
	}
	reqLen := binary.BigEndian.Uint32(header)
	reqBytes := make([]byte, reqLen)
	if _, err := io.ReadFull(stream, reqBytes); err != nil {
		logError("Error reading stream request: %v", err)
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(reqBytes)))
	if err != nil {
		logError("Error parsing HTTP request: %v", err)
		return
	}

	// Preserve client IP if available
	clientIP := req.Header.Get("X-Forwarded-For")
	// Get remote address for logging purposes
	remoteAddrStr := req.RemoteAddr
	req.RequestURI = ""
	req.URL.Scheme = "http"
	req.URL.Host = local

	if preserveClientIP && clientIP != "" {
		logInfo("Preserving client IP: %s", clientIP)
	}

	// Only log non-HMR requests to reduce noise
	if !strings.Contains(req.URL.Path, "/_next/webpack-hmr") {
		sourceIP := clientIP
		if sourceIP == "" {
			sourceIP = remoteAddrStr
		}
		logSuccess("Request: %s %s (from %s)", req.Method, req.URL.Path, sourceIP)
	}

	// Forward the request to the local service
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logError("Local forward failed: %v", err)
		resp = &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("Failed to forward to local service")),
			Header:     make(http.Header),
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
	} else {
		// Log successful responses but not for HMR
		if !strings.Contains(req.URL.Path, "/_next/webpack-hmr") {
			logSuccess("Response: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
	}

	// Write the response back to the stream with a 4-byte length header.
	var buf bytes.Buffer
	if err := resp.Write(&buf); err != nil {
		logError("Error encoding response: %v", err)
		return
	}
	respBytes := buf.Bytes()
	respLen := uint32(len(respBytes))
	lengthHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthHeader, respLen)

	// Set a write deadline for safety.
	stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := stream.Write(append(lengthHeader, respBytes...)); err != nil {
		logError("Error sending response on stream: %v", err)
		return
	}
	stream.SetWriteDeadline(time.Time{})
}
