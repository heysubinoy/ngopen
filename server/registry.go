package server

import (
	"log"
	"net"
	"sync"

	"github.com/xtaci/smux"
)

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