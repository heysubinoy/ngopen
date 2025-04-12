package main

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
	// "ngopen"
)

type TunnelRegistry struct {
	sync.RWMutex
	clients map[string]net.Conn
}

func NewTunnelRegistry() *TunnelRegistry {
	return &TunnelRegistry{
		clients: make(map[string]net.Conn),
	}
}

func (r *TunnelRegistry) Add(name string, conn net.Conn) {
	r.Lock()
	defer r.Unlock()
	r.clients[name] = conn
	log.Printf("Tunnel client '%s' registered.\n", name)
}

func (r *TunnelRegistry) Get(name string) (net.Conn, bool) {
	r.RLock()
	defer r.RUnlock()
	conn, ok := r.clients[name]
	return conn, ok
}

func hashSHA1(s string) string {
	hash := sha1.New()
	hash.Write([]byte(s))
	hashedValue := hash.Sum(nil)
	// name = string(hashedValue)
	return string(hashedValue)
}

func startTunnelListener(registry *TunnelRegistry) {
	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal("Tunnel listen error:", err)
	}
	log.Println("Waiting for tunnel clients on :9000...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		log.Println("Tunnel client connected:", conn.RemoteAddr())

		go func(c net.Conn) {
			fmt.Println("Parsing Request", c.RemoteAddr())

			// Set read deadline (optional, for safety)
			c.SetReadDeadline(time.Now().Add(5 * time.Second))

			// Parse directly from conn
			message, err := ParseProtocolMessage(c)
			if err != nil {
				log.Println("Failed to parse protocol message:", err)
				c.Close()
				return
			}

			name := message.RemoteAddr
			if name == "" {
				log.Println("Parsed name is empty, closing connection.")
				c.Close()
				return
			}
			fmt.Println("Parsed name:", name)
			registry.Add(name, c)
			fmt.Println("Tunnel client registered:", name)
		}(conn)

	}
}

func startHTTPServer(registry *TunnelRegistry) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := r.Host
		if target == "" {
			http.Error(w, "Missing Host header", http.StatusBadRequest)
			return
		}
		fmt.Printf("Received request for target: %s\n", target)

		conn, ok := registry.Get(target)
		if !ok {
			http.Error(w, "Tunnel client not connected", http.StatusServiceUnavailable)
			return
		}

		// Create a mutex for this connection
		connMutex := &sync.Mutex{}

		// Lock the connection for this request/response cycle
		connMutex.Lock()
		defer connMutex.Unlock()

		// Forward the request
		err := r.Write(conn)
		if err != nil {
			log.Println("Failed to write to tunnel:", err)
			http.Error(w, "Tunnel write failed", http.StatusBadGateway)
			return
		}

		// Set read deadline for the response
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		// Read response back
		resp, err := http.ReadResponse(bufio.NewReader(conn), r)
		if err != nil {
			log.Println("Failed to read from tunnel:", err)
			http.Error(w, "Tunnel response failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Clear the read deadline
		conn.SetReadDeadline(time.Time{})

		// Copy headers and body
		for k, v := range resp.Header {
			for _, val := range v {
				w.Header().Add(k, val)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	log.Println("HTTP server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	registry := NewTunnelRegistry()

	go startTunnelListener(registry)
	startHTTPServer(registry)
}
