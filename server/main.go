// server/main.go
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xtaci/smux"
)

// Client represents a tunnel client with an associated smux session.
type Client struct {
	Conn    net.Conn
	Session *smux.Session
	Name    string
}

type TunnelRegistry struct {
	sync.RWMutex
	clients map[string]*Client
}

func NewTunnelRegistry() *TunnelRegistry {
	return &TunnelRegistry{
		clients: make(map[string]*Client),
	}
}

func (r *TunnelRegistry) Add(name string, client *Client) {
	r.Lock()
	defer r.Unlock()
	r.clients[name] = client
	log.Printf("Tunnel client '%s' registered.", name)
}

func (r *TunnelRegistry) Get(name string) (*Client, bool) {
	r.RLock()
	defer r.RUnlock()
	client, ok := r.clients[name]
	return client, ok
}

func (r *TunnelRegistry) Remove(name string) {
	r.Lock()
	defer r.Unlock()
	if client, ok := r.clients[name]; ok {
		client.Conn.Close()
		client.Session.Close()
		delete(r.clients, name)
		log.Printf("Tunnel client '%s' unregistered.", name)
	}
}

func generateHostname() string {
	rand.Seed(time.Now().UnixNano())
	adjectives := []string{"red", "blue", "happy", "swift", "clever", "brave", "kind", "wise", "calm", "bold"}
	nouns := []string{"fox", "bear", "eagle", "wolf", "tiger", "lion", "hawk", "deer", "snake", "panda"}
	return fmt.Sprintf("%s-%s-%d.n.sbn.lol", adjectives[rand.Intn(len(adjectives))], nouns[rand.Intn(len(nouns))], rand.Intn(1000))
}

// startTunnelListener listens for new tunnel client connections.
func startTunnelListener(registry *TunnelRegistry) {
	// Set your desired auth token - you could also load this from an environment variable
	const validAuthToken = "your-secure-token-here" // Change this to your secure token

	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal("Tunnel listen error:", err)
	}
	log.Println("Listening for tunnel clients on :9000…")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go func(c net.Conn) {
			reader := bufio.NewReader(c)

			// Read and validate auth token
			authToken, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Failed to read auth token:", err)
				c.Close()
				return
			}
			authToken = strings.TrimSpace(authToken)

			// Validate the token
			if authToken != validAuthToken {
				log.Printf("Invalid auth token '%s', closing connection.", authToken)
				c.Close()
				return
			}
			log.Println("Client authenticated successfully")

			// Read client message (hostname request)
			clientMsg, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Failed to read client message:", err)
				c.Close()
				return
			}
			clientMsg = strings.TrimSpace(clientMsg)
			var name string
			if clientMsg == "AUTO" {
				name = generateHostname()
				if _, err := fmt.Fprintf(c, "%s\n", name); err != nil {
					log.Println("Failed to send hostname to client:", err)
					c.Close()
					return
				}
			} else {
				name = clientMsg
			}
			if name == "" {
				log.Println("Empty client name, closing.")
				c.Close()
				return
			}

			// Create a smux session for this client connection.
			session, err := smux.Server(c, nil)
			if err != nil {
				log.Println("Failed to create smux session:", err)
				c.Close()
				return
			}

			client := &Client{
				Conn:    c,
				Session: session,
				Name:    name,
			}
			registry.Add(name, client)
			log.Printf("Tunnel client '%s' connected.", name)

			// Block until the session is closed.
			<-session.CloseChan()
			registry.Remove(name)
		}(conn)
	}
}

// writeFramedRequest writes an HTTP request into the given stream.
func writeFramedRequest(stream net.Conn, req *http.Request) error {
	var buf bytes.Buffer
	if err := req.Write(&buf); err != nil {
		return err
	}
	data := buf.Bytes()
	length := uint32(len(data))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)
	_, err := stream.Write(append(header, data...))
	return err
}

// readFramedResponse reads a framed HTTP response from the given stream.
func readFramedResponse(stream net.Conn, req *http.Request) (*http.Response, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(stream, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	body := make([]byte, length)
	if _, err := io.ReadFull(stream, body); err != nil {
		return nil, err
	}
	return http.ReadResponse(bufio.NewReader(bytes.NewReader(body)), req)
}

// startHTTPServer starts an HTTP server that, on each request, opens a new smux stream.
func startHTTPServer(registry *TunnelRegistry) {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := r.Host
		if target == "" {
			http.Error(w, "Missing Host header", http.StatusBadRequest)
			return
		}
		
		// Filter out hot module reload chatter.
		if !strings.Contains(r.URL.Path, "/_next/webpack-hmr") {
			log.Printf("Request: %s %s", r.Method, r.URL.Path)
		}

		tunnelClient, ok := registry.Get(target)
		if !ok {
			http.Error(w, "Tunnel client not connected", http.StatusServiceUnavailable)
			return
		}

		// Open a new stream for this HTTP request.
		stream, err := tunnelClient.Session.OpenStream()
		if err != nil {
			log.Println("Failed to open smux stream:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel stream open failed", http.StatusBadGateway)
			return
		}
		defer stream.Close()

		// Write the request over the stream.
		stream.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if err := writeFramedRequest(stream, r); err != nil {
			log.Println("Failed to write to tunnel stream:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel write failed", http.StatusBadGateway)
			return
		}
		stream.SetReadDeadline(time.Now().Add(30 * time.Second))
		resp, err := readFramedResponse(stream, r)
		if err != nil {
			log.Println("Failed to read from tunnel stream:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel response failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		stream.SetWriteDeadline(time.Time{})
		stream.SetReadDeadline(time.Time{})

		// Copy response headers and body.
		for k, vals := range resp.Header {
			w.Header()[k] = vals
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	server := &http.Server{
		Addr:           ":443",
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	log.Println("HTTPS server listening on :443")
	log.Fatal(server.ListenAndServeTLS("/etc/letsencrypt/live/n.sbn.lol/fullchain.pem", "/etc/letsencrypt/live/n.sbn.lol/privkey.pem"))
}

func main() {
	registry := NewTunnelRegistry()
	go startTunnelListener(registry)
	startHTTPServer(registry)
}
