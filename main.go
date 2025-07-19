package main

import (
	"os"
	"strings"

	"github.com/heysubinoy/ngopen/client"
	"github.com/heysubinoy/ngopen/server"
)

func main() {
	if len(os.Args) > 1 && strings.ToLower(os.Args[1]) == "server" {
		registry := server.NewTunnelRegistry()
		go server.StartTunnelListener(registry)
		server.StartHTTPServer(registry)
	} else {
		client.Main()
	}
}
