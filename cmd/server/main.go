// server/main.go
package main

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/heysubinoy/ngopen/protocol"
	"github.com/heysubinoy/ngopen/server"

	"github.com/xtaci/smux"
)

func authenticate(stream net.Conn) (string, bool) {
	msg, err := protocol.DecodeProtocolAuthMessage(stream)
	if err != nil {
		server.LogError("Failed to decode auth message: %v", err)
		return "", false
	}
	if !server.IsValidToken(msg.AuthToken) {
		resp := "FAIL:Invalid token"
		respHeader := make([]byte, 4)
		binary.BigEndian.PutUint32(respHeader, uint32(len(resp)))
		stream.Write(append(respHeader, []byte(resp)...))
		return "", false
	}
	assigned := msg.Hostname
	if assigned == "AUTO" || assigned == "" {
		assigned = server.GenerateHostname()
	}
	resp := "OK:" + assigned
	respHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(respHeader, uint32(len(resp)))
	stream.Write(append(respHeader, []byte(resp)...))
	return assigned, true
}

func startTunnelListener(registry *server.TunnelRegistry) {
	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		server.LogError("Tunnel listen error:", err)
	}
	server.LogInfo("Listening for tunnel clients on :9000…")

	for {
		conn, err := ln.Accept()
		if err != nil {
			server.LogError("Accept error:", err)
			continue
		}
		go func(c net.Conn) {
			session, err := smux.Server(c, nil)
			if err != nil {
				server.LogError("smux session error:", err)
				c.Close()
				return
			}
			// Use the first stream for authentication only
			authStream, err := session.AcceptStream()
			if err != nil {
				server.LogError("Failed to accept auth stream:", err)
				session.Close()
				return
			}
			assignedHostname, ok := authenticate(authStream)
			authStream.Close()
			if !ok {
				server.LogError("Authentication failed, closing session")
				session.Close()
				return
			}
			client := &server.Client{
				Conn:    c,
				Session: session,
				Name:    assignedHostname,
			}
			registry.Add(assignedHostname, client)
			server.LogInfo("Tunnel client '%s' connected.", assignedHostname)
			<-session.CloseChan()
			registry.Remove(assignedHostname)
		}(conn)
	}
}

// startHTTPServer starts an HTTP server that, on each request, opens a new smux stream.
func startHTTPServer(registry *server.TunnelRegistry) {
	devMode := os.Getenv("NGOPEN_MODE") == "DEV"
	addr := ":8080"
	if devMode {
		addr = ":8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := r.Host
		if target == "" {
			http.Error(w, "Missing Host header", http.StatusBadRequest)
			return
		}

		// Filter out hot module reload chatter.
		if !strings.Contains(r.URL.Path, "/_next/webpack-hmr") {
			server.LogInfo("Request: %s %s", r.Method, r.URL.Path)
		}

		tunnelClient, ok := registry.Get(target)
		if !ok {
			http.Error(w, "Tunnel client not connected", http.StatusServiceUnavailable)
			return
		}

		// Open a new stream for this HTTP request.
		stream, err := tunnelClient.Session.OpenStream()
		if err != nil {
			server.LogError("Failed to open smux stream:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel stream open failed", http.StatusBadGateway)
			return
		}
		defer stream.Close()

		// Write the request over the stream.
		stream.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if err := server.WriteFramedRequest(stream, r); err != nil {
			server.LogError("Failed to write to tunnel stream:", err)
			registry.Remove(target)
			http.Error(w, "Tunnel write failed", http.StatusBadGateway)
			return
		}
		stream.SetReadDeadline(time.Now().Add(30 * time.Second))
		resp, err := server.ReadFramedResponse(stream, r)
		if err != nil {
			server.LogError("Failed to read from tunnel stream:", err)
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

	serve := &http.Server{
		Addr:           addr,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if devMode {
		server.LogInfo("HTTP server (dev mode) listening on %s", addr)
		log.Fatal(serve.ListenAndServe())
	} else {
		// certFile := os.Getenv("NGOPEN_CERT_FILE")
		// keyFile := os.Getenv("NGOPEN_KEY_FILE")
		// if certFile == "" {
		// 	certFile = "/etc/letsencrypt/live/n.sbn.lol/fullchain.pem"
		// }
		// if keyFile == "" {
		// 	keyFile = "/etc/letsencrypt/live/n.sbn.lol/privkey.pem"
		// } 
		//Will be handled by the reverse proxy in production
		server.LogInfo("HTTPS server (prod mode) listening on %s", addr)
		log.Fatal(serve.ListenAndServe())
	}
}

func main() {
	registry := server.NewTunnelRegistry()
	go startTunnelListener(registry)
	startHTTPServer(registry)
}
