package main

import (
	"bufio"
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
)

type Client struct {
	Conn  net.Conn
	Mutex sync.Mutex
	Name  string
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
	log.Printf("Tunnel client '%s' registered.\n", name)
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
		delete(r.clients, name)
		log.Printf("Tunnel client '%s' unregistered.\n", name)
	}
}

// generateHostname creates a unique hostname for the client
func generateHostname() string {
	rand.Seed(time.Now().UnixNano())
	adjectives := []string{"red", "blue", "happy", "swift", "clever", "brave", "kind", "wise", "calm", "bold"}
	nouns := []string{"fox", "bear", "eagle", "wolf", "tiger", "lion", "hawk", "deer", "snake", "panda"}

	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	number := rand.Intn(1000)

	return fmt.Sprintf("%s-%s-%d", adj, noun, number)
}

// --- Multiplexed Tunnel Listener ---

func startTunnelListener(registry *TunnelRegistry) {
	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal("Tunnel listen error:", err)
	}
	log.Println("Listening for tunnel clients on :9000...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		go func(c net.Conn) {
			reader := bufio.NewReader(c)
			clientMsg, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Failed to read client message:", err)
				c.Close()
				return
			}
			clientMsg = strings.TrimSpace(clientMsg)

			// If client sends "AUTO", generate hostname
			// Otherwise use provided name
			var name string
			if clientMsg == "AUTO" {
				name = generateHostname()+":8080"
				// Send the generated hostname back to the client
				_, err = fmt.Fprintf(c, "%s\n", name)
				if err != nil {
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

			client := &Client{
				Conn:  c,
				Name:  name,
				Mutex: sync.Mutex{},
			}

			registry.Add(name, client)

			// No read loop here. Tunnel is passive; it replies to request proxies.

			log.Printf("Tunnel client '%s' connected.\n", name)
		}(conn)
	}
}

// --- Framing Utils ---

func writeFramedRequest(conn net.Conn, req *http.Request) error {
	var buf strings.Builder
	err := req.Write(&buf)
	if err != nil {
		return err
	}
	data := []byte(buf.String())

	length := uint32(len(data))
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, length)

	_, err = conn.Write(append(header, data...))
	return err
}

func readFramedResponse(conn net.Conn, req *http.Request) (*http.Response, error) {
	header := make([]byte, 4)
	_, err := io.ReadFull(conn, header)
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)
	body := make([]byte, length)
	_, err = io.ReadFull(conn, body)
	if err != nil {
		return nil, err
	}

	return http.ReadResponse(bufio.NewReader(strings.NewReader(string(body))), req)
}

// --- HTTP Server Side ---

func startHTTPServer(registry *TunnelRegistry) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := r.Host
		if target == "" {
			http.Error(w, "Missing Host header", http.StatusBadRequest)
			return
		}
		log.Printf("Incoming request for %s\n", target)

		client, ok := registry.Get(target)
		if !ok {
			http.Error(w, "Tunnel client not connected", http.StatusServiceUnavailable)
			return
		}

		client.Mutex.Lock()
		defer client.Mutex.Unlock()

		err := writeFramedRequest(client.Conn, r)
		if err != nil {
			log.Println("Failed to write to tunnel:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel write failed", http.StatusBadGateway)
			return
		}

		resp, err := readFramedResponse(client.Conn, r)
		if err != nil {
			log.Println("Failed to read from tunnel:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel response failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})

	log.Println("HTTP server listening on :80")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	registry := NewTunnelRegistry()

	go startTunnelListener(registry)
	startHTTPServer(registry)
}
